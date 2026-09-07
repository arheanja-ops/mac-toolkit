package docker

import (
	"encoding/json"
	"fmt"
	"github.com/arheanja-ops/mac-toolkit/internal/core"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BackupResult holds the result of a container DB backup
type BackupResult struct {
	Path           string `json:"path"`
	SizeBytes      int64  `json:"size_bytes"`
	Container      string `json:"container"`
	Engine         string `json:"engine"`
	User           string `json:"user"`
	DatabasesFound int    `json:"databases_found"`
	TablesFound    int    `json:"tables_found"`
	Verified       bool   `json:"verified"`
	Error          string `json:"error,omitempty"`
}

// CleanupResult holds the result of a selective docker cleanup
type CleanupResult struct {
	DeletedContainers []string `json:"deleted_containers"`
	DeletedImages     []string `json:"deleted_images"`
	DeletedVolumes    []string `json:"deleted_volumes"`
	BuildCacheFreed   string   `json:"build_cache_freed"`
	Kept              []string `json:"kept"`
	DryRun            bool     `json:"dry_run"`
	Error             string   `json:"error,omitempty"`
}

// CompactResult holds Docker disk compaction analysis
type CompactResult struct {
	RawSize          int64  `json:"raw_size_bytes"`
	RawSizeHuman     string `json:"raw_size"`
	ActualUsed       int64  `json:"actual_used_bytes"`
	ActualUsedHuman  string `json:"actual_used"`
	RecommendedLimit int64  `json:"recommended_limit_bytes"`
	RecommendedHuman string `json:"recommended_limit"`
	Instructions     string `json:"instructions"`
	Error            string `json:"error,omitempty"`
}

// ContainerInfo holds parsed container metadata
type ContainerInfo struct {
	Name    string
	Image   string
	Status  string
	Volumes []string
	Env     map[string]string
}

// Backup performs a database backup of a running container
func Backup(container string) BackupResult {
	// Check container is running
	info, err := inspectContainer(container)
	if err != nil {
		return BackupResult{Container: container, Error: fmt.Sprintf("inspect failed: %v", err)}
	}
	if !strings.Contains(strings.ToLower(info.Status), "up") {
		return BackupResult{Container: container, Error: fmt.Sprintf("container not running (status: %s)", info.Status)}
	}

	// Detect engine
	engine := detectEngine(info.Image)
	if engine == "" {
		return BackupResult{Container: container, Error: fmt.Sprintf("unsupported engine for image %q — only postgres and mysql/mariadb supported", info.Image)}
	}

	// Build dump command
	user := info.Env["POSTGRES_USER"]
	if user == "" {
		user = info.Env["MYSQL_USER"]
	}
	if user == "" && engine == "postgres" {
		user = "postgres" // default
	}
	if user == "" && engine == "mysql" {
		user = "root"
	}

	var dumpCmd []string
	var trailer string
	switch engine {
	case "postgres":
		dumpCmd = []string{"pg_dumpall", "-U", user}
		trailer = "dump complete"
	case "mysql":
		pass := info.Env["MYSQL_ROOT_PASSWORD"]
		if pass == "" {
			pass = info.Env["MYSQL_PASSWORD"]
		}
		dumpCmd = []string{"mysqldump", "--all-databases", fmt.Sprintf("-u%s", user)}
		if pass != "" {
			dumpCmd = append(dumpCmd, fmt.Sprintf("-p%s", pass))
		}
		trailer = "Dump completed"
	}

	// Prepare output path
	backupDir := filepath.Join(core.HomeDir, "docker-backups")
	os.MkdirAll(backupDir, 0755)
	timestamp := time.Now().Format("2006-01-02-150405")
	outPath := filepath.Join(backupDir, fmt.Sprintf("%s-%s.sql", container, timestamp))

	// Execute dump
	args := append([]string{"exec", container}, dumpCmd...)
	cmd := exec.Command("docker", args...)
	outFile, err := os.Create(outPath)
	if err != nil {
		return BackupResult{Container: container, Engine: engine, Error: fmt.Sprintf("create file: %v", err)}
	}
	cmd.Stdout = outFile
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		outFile.Close()
		os.Remove(outPath)
		return BackupResult{Container: container, Engine: engine, User: user, Error: fmt.Sprintf("dump failed: %v — %s", err, stderr.String())}
	}
	outFile.Close()

	// Verify
	stat, _ := os.Stat(outPath)
	size := stat.Size()
	if size == 0 {
		os.Remove(outPath)
		return BackupResult{Container: container, Engine: engine, User: user, Error: "dump file is empty"}
	}

	// Read last 200 bytes to check trailer
	verified := false
	if f, err := os.Open(outPath); err == nil {
		defer f.Close()
		tailBuf := make([]byte, 200)
		if size > 200 {
			f.Seek(size-200, 0)
		}
		n, _ := f.Read(tailBuf)
		tail := string(tailBuf[:n])
		verified = strings.Contains(strings.ToLower(tail), strings.ToLower(trailer))
	}

	// Count databases and tables
	content, _ := os.ReadFile(outPath)
	sql := string(content)
	dbCount := strings.Count(sql, "CREATE DATABASE")
	tblCount := strings.Count(sql, "CREATE TABLE")

	return BackupResult{
		Path:           outPath,
		SizeBytes:      size,
		Container:      container,
		Engine:         engine,
		User:           user,
		DatabasesFound: dbCount,
		TablesFound:    tblCount,
		Verified:       verified,
	}
}

// Cleanup removes all Docker resources except the kept containers and their deps
func Cleanup(keepNames []string, dryRun bool) CleanupResult {
	result := CleanupResult{DryRun: dryRun, Kept: keepNames}

	// Get all containers
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}").Output()
	if err != nil {
		result.Error = fmt.Sprintf("docker ps failed: %v", err)
		return result
	}
	allContainers := nonEmpty(strings.Split(strings.TrimSpace(string(out)), "\n"))

	// Build keep set
	keepSet := make(map[string]bool)
	for _, k := range keepNames {
		keepSet[strings.TrimSpace(k)] = true
	}

	// Find images and volumes to keep
	keepImages := make(map[string]bool)
	keepVolumes := make(map[string]bool)
	for _, name := range keepNames {
		info, err := inspectContainer(name)
		if err != nil {
			continue
		}
		keepImages[info.Image] = true
		for _, v := range info.Volumes {
			keepVolumes[v] = true
		}
	}

	// Containers to delete
	for _, c := range allContainers {
		if !keepSet[c] {
			result.DeletedContainers = append(result.DeletedContainers, c)
			if !dryRun {
				exec.Command("docker", "rm", "-f", c).Run()
			}
		}
	}

	// Images to delete
	imgOut, _ := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}").Output()
	allImages := nonEmpty(strings.Split(strings.TrimSpace(string(imgOut)), "\n"))
	for _, img := range allImages {
		if !keepImages[img] {
			// Also check without tag
			repo := strings.Split(img, ":")[0]
			if !keepImages[repo] {
				result.DeletedImages = append(result.DeletedImages, img)
				if !dryRun {
					exec.Command("docker", "rmi", img).Run()
				}
			}
		}
	}

	// Volumes to delete
	volOut, _ := exec.Command("docker", "volume", "ls", "-q").Output()
	allVolumes := nonEmpty(strings.Split(strings.TrimSpace(string(volOut)), "\n"))
	for _, v := range allVolumes {
		if !keepVolumes[v] {
			result.DeletedVolumes = append(result.DeletedVolumes, v)
			if !dryRun {
				exec.Command("docker", "volume", "rm", v).Run()
			}
		}
	}

	// Build cache
	if !dryRun {
		out, err := exec.Command("docker", "builder", "prune", "-a", "-f").Output()
		if err == nil {
			result.BuildCacheFreed = extractReclaimedSize(string(out))
		}
	} else {
		// Get reclaimable size for preview
		out, _ := exec.Command("docker", "system", "df", "--format", "{{.Type}}\t{{.Reclaimable}}").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Build Cache") {
				parts := strings.Split(line, "\t")
				if len(parts) > 1 {
					result.BuildCacheFreed = parts[1] + " (reclaimable)"
				}
			}
		}
	}

	return result
}

// Compact analyzes Docker.raw vs actual usage and recommends a new disk limit
func Compact() CompactResult {
	rawPath := core.DockerRawPath
	info, err := os.Stat(rawPath)
	if os.IsNotExist(err) {
		return CompactResult{Error: "Docker.raw not found — Docker Desktop may not be installed"}
	}
	if err != nil {
		return CompactResult{Error: fmt.Sprintf("stat error: %v", err)}
	}

	rawSize := info.Size()

	// Get actual usage from docker system df
	var actualUsed int64
	out, err := exec.Command("docker", "system", "df", "--format", "json").Output()
	if err != nil {
		return CompactResult{
			RawSize:      rawSize,
			RawSizeHuman: core.FormatBytes(rawSize),
			Error:        "Docker daemon not running — start Docker Desktop first",
			Instructions: "1. Start Docker Desktop\n2. Run 'toolkit docker compact' again",
		}
	}

	// Parse each line of JSON (docker outputs one JSON object per type)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var entry struct {
			Size string `json:"Size"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil {
			actualUsed += parseDockerSize(entry.Size)
		}
	}

	// Recommended: max(actual * 2, 16 GB)
	recommended := actualUsed * 2
	minLimit := int64(16) * 1024 * 1024 * 1024
	if recommended < minLimit {
		recommended = minLimit
	}

	instructions := fmt.Sprintf(
		"Docker.raw is %s on disk but only %s is actually used.\n\n"+
			"To reclaim space:\n"+
			"1. Open Docker Desktop → Settings → Resources\n"+
			"2. Set 'Disk usage limit' to %s (currently at %s)\n"+
			"3. Click 'Apply & Restart'\n\n"+
			"⚠️  This recreates the virtual disk — all containers, images, and volumes are lost.\n"+
			"Run 'toolkit docker backup <container>' first for any container with a database.",
		core.FormatBytes(rawSize),
		core.FormatBytes(actualUsed),
		core.FormatBytes(recommended),
		core.FormatBytes(rawSize),
	)

	return CompactResult{
		RawSize:          rawSize,
		RawSizeHuman:     core.FormatBytes(rawSize),
		ActualUsed:       actualUsed,
		ActualUsedHuman:  core.FormatBytes(actualUsed),
		RecommendedLimit: recommended,
		RecommendedHuman: core.FormatBytes(recommended),
		Instructions:     instructions,
	}
}

// --- helpers ---

func inspectContainer(name string) (ContainerInfo, error) {
	out, err := exec.Command("docker", "inspect", name).Output()
	if err != nil {
		return ContainerInfo{}, fmt.Errorf("docker inspect %s: %w", name, err)
	}

	var inspects []struct {
		Name   string `json:"Name"`
		State  struct {
			Status string `json:"Status"`
		} `json:"State"`
		Config struct {
			Image string   `json:"Image"`
			Env   []string `json:"Env"`
		} `json:"Config"`
		Mounts []struct {
			Type string `json:"Type"`
			Name string `json:"Name"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(out, &inspects); err != nil {
		return ContainerInfo{}, err
	}
	if len(inspects) == 0 {
		return ContainerInfo{}, fmt.Errorf("container %q not found", name)
	}

	c := inspects[0]
	env := make(map[string]string)
	for _, e := range c.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	var volumes []string
	for _, m := range c.Mounts {
		if m.Type == "volume" && m.Name != "" {
			volumes = append(volumes, m.Name)
		}
	}

	return ContainerInfo{
		Name:    strings.TrimPrefix(c.Name, "/"),
		Image:   c.Config.Image,
		Status:  c.State.Status,
		Volumes: volumes,
		Env:     env,
	}, nil
}

func detectEngine(image string) string {
	img := strings.ToLower(image)
	if strings.Contains(img, "postgres") {
		return "postgres"
	}
	if strings.Contains(img, "mysql") || strings.Contains(img, "mariadb") {
		return "mysql"
	}
	return ""
}

func nonEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

var sizeRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB|kB)`)

func parseDockerSize(s string) int64 {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var val float64
	fmt.Sscanf(m[1], "%f", &val)
	switch strings.ToUpper(m[2]) {
	case "TB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GB":
		return int64(val * 1024 * 1024 * 1024)
	case "MB":
		return int64(val * 1024 * 1024)
	case "KB":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}

func extractReclaimedSize(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(strings.ToLower(line), "reclaimed") {
			return strings.TrimSpace(line)
		}
	}
	return "unknown"
}
