package core

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ApprovalEngine handles user approval for cleanup operations
type ApprovalEngine struct {
	Mode    ApprovalMode
	DryRun  bool
	Execute bool
	reader  *bufio.Reader
}

// NewApprovalEngine creates a new approval engine
func NewApprovalEngine(mode ApprovalMode, dryRun, execute bool) *ApprovalEngine {
	return &ApprovalEngine{
		Mode:    mode,
		DryRun:  dryRun,
		Execute: execute && !dryRun,
		reader:  bufio.NewReader(os.Stdin),
	}
}

// FilterItems applies the approval mode to filter items the user wants to delete.
// Returns only the approved items.
func (a *ApprovalEngine) FilterItems(results []AnalysisResult) []CleanableItem {
	if a.DryRun || !a.Execute {
		Info("Dry-run mode — no files will be deleted")
		return nil
	}

	switch a.Mode {
	case ApprovalDeal:
		return a.dealMode(results)
	case ApprovalCategory:
		return a.categoryMode(results)
	case ApprovalItem:
		return a.itemMode(results)
	case ApprovalChecklist:
		return a.checklistMode(results)
	default:
		return a.dealMode(results)
	}
}

func (a *ApprovalEngine) dealMode(results []AnalysisResult) []CleanableItem {
	// Show total reclaimable
	var totalSize int64
	var totalItems int
	for _, r := range results {
		for _, item := range r.Items {
			if item.SafeToDelete {
				totalSize += item.SizeBytes
				totalItems++
			}
		}
	}

	if totalItems == 0 {
		Info("No safe items to clean")
		return nil
	}

	Bold("\nTotal reclaimable: %s (%d items)", FormatBytes(totalSize), totalItems)
	if !a.confirm("Ready to review by category?") {
		return nil
	}

	// Then approve per category
	return a.categoryMode(results)
}

func (a *ApprovalEngine) categoryMode(results []AnalysisResult) []CleanableItem {
	var approved []CleanableItem

	for _, r := range results {
		safeItems := filterSafe(r.Items)
		if len(safeItems) == 0 {
			continue
		}

		// Sort by size descending
		sort.Slice(safeItems, func(i, j int) bool {
			return safeItems[i].SizeBytes > safeItems[j].SizeBytes
		})

		var domainSize int64
		for _, item := range safeItems {
			domainSize += item.SizeBytes
		}

		prompt := fmt.Sprintf("Clean %s — %s (%d items)?", r.Domain, FormatBytes(domainSize), len(safeItems))
		if a.confirm(prompt) {
			approved = append(approved, safeItems...)
		}
	}

	return approved
}

func (a *ApprovalEngine) itemMode(results []AnalysisResult) []CleanableItem {
	var approved []CleanableItem

	for _, r := range results {
		for _, item := range r.Items {
			if !item.SafeToDelete {
				continue
			}
			prompt := fmt.Sprintf("Delete %s (%s) [%s]?", item.Label, FormatBytes(item.SizeBytes), item.Path)
			if a.confirm(prompt) {
				approved = append(approved, item)
			}
		}
	}

	return approved
}

func (a *ApprovalEngine) checklistMode(results []AnalysisResult) []CleanableItem {
	// Collect all safe items
	var allItems []CleanableItem
	for _, r := range results {
		for _, item := range r.Items {
			if item.SafeToDelete {
				allItems = append(allItems, item)
			}
		}
	}

	if len(allItems) == 0 {
		Info("No safe items to clean")
		return nil
	}

	// Show numbered list
	Bold("\nSelect items to delete (enter numbers separated by commas, or 'all'):")
	for i, item := range allItems {
		fmt.Fprintf(os.Stderr, "  %s[%d]%s %s — %s (%s)\n",
			colorCyan, i+1, colorReset,
			item.Domain, item.Label, FormatBytes(item.SizeBytes))
	}

	fmt.Fprint(os.Stderr, "\n> ")
	input, err := a.reader.ReadString('\n')
	if err != nil {
		return nil
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	if strings.EqualFold(input, "all") || strings.EqualFold(input, "todos") {
		return allItems
	}

	// Parse comma-separated numbers
	var approved []CleanableItem
	for _, s := range strings.Split(input, ",") {
		s = strings.TrimSpace(s)
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > len(allItems) {
			continue
		}
		approved = append(approved, allItems[n-1])
	}

	return approved
}

// confirm asks a yes/no question and accepts Spanish + English
func (a *ApprovalEngine) confirm(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s [s/N]: ", prompt)
	input, err := a.reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "s", "si", "sí", "y", "yes":
		return true
	default:
		return false
	}
}

// filterSafe returns only items marked safe to delete
func filterSafe(items []CleanableItem) []CleanableItem {
	var safe []CleanableItem
	for _, item := range items {
		if item.SafeToDelete {
			safe = append(safe, item)
		}
	}
	return safe
}
