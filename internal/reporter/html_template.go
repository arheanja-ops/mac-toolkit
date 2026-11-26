package reporter

// htmlTemplate is the whole page: CSS inline, no script, no remote resource.
// It is parsed by html/template, which escapes every {{ }} value for its
// context (text, attribute, CSS), so report data can never inject markup.
const htmlTemplate = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Reporte de Mac Toolkit</title>
<style>
:root{color-scheme:light;--ink:#1f2933;--muted:#616e7c;--line:#d9e0e7;--bg:#f4f6f8;--card:#fff;
--ok:#2f7d4f;--okbg:#e6f3ec;--warn:#9a6a00;--warnbg:#fbf1d9;--crit:#b3261e;--critbg:#fbe4e2;--accent:#3b5b8c}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.5 -apple-system,"Segoe UI",Helvetica,Arial,sans-serif}
main{max-width:980px;margin:0 auto;padding:24px 20px 48px}
h1{font-size:22px;margin:0 0 4px}
h2{font-size:16px;margin:32px 0 12px;padding-bottom:6px;border-bottom:2px solid var(--line)}
h3{font-size:14px;margin:20px 0 8px;color:var(--muted);text-transform:uppercase;letter-spacing:.04em}
.sub{color:var(--muted);margin:0 0 16px}
.notice{background:var(--warnbg);border:1px solid #ecd9a6;border-radius:6px;padding:8px 12px;margin:12px 0}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px;margin:12px 0}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:12px 14px}
.card .k{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.card .v{font-size:22px;font-weight:600;margin-top:2px}
.card .n{color:var(--muted);font-size:12px}
.badge{display:inline-block;border-radius:999px;padding:2px 10px;font-weight:600;font-size:12px}
.ok{color:var(--ok)}.warn{color:var(--warn)}.crit{color:var(--crit)}
.badge.ok{background:var(--okbg)}.badge.warn{background:var(--warnbg)}.badge.crit{background:var(--critbg)}
.card.ok{border-left:4px solid var(--ok)}.card.warn{border-left:4px solid var(--warn)}.card.crit{border-left:4px solid var(--crit)}
table{width:100%;border-collapse:collapse;background:var(--card);border:1px solid var(--line);border-radius:6px;margin:8px 0 16px}
thead th{position:sticky;top:0;background:#e9eef3;text-align:left;font-size:12px;color:var(--muted);
text-transform:uppercase;letter-spacing:.04em;padding:8px 10px;border-bottom:2px solid var(--line)}
td{padding:7px 10px;border-bottom:1px solid var(--line);vertical-align:top;overflow-wrap:anywhere}
tbody tr:last-child td{border-bottom:0}
.num{text-align:right;white-space:nowrap;font-variant-numeric:tabular-nums}
code{font:12px ui-monospace,Menlo,Consolas,monospace;color:var(--muted)}
.bar{display:inline-block;width:120px;height:8px;background:var(--line);border-radius:4px;overflow:hidden;vertical-align:middle}
.bar>span{display:block;height:100%;background:var(--accent)}
.bar.ok>span{background:var(--ok)}.bar.warn>span{background:var(--warn)}.bar.crit>span{background:var(--crit)}
.more{color:var(--muted);margin:-8px 0 16px}
footer{margin-top:32px;color:var(--muted);font-size:12px}
@media (max-width:600px){main{padding:16px 10px}.bar{width:70px}}
@media print{body{background:#fff;font-size:11px}main{max-width:none;padding:0}
.card,table{break-inside:avoid}h2{break-after:avoid}thead th{position:static}
thead{display:table-header-group}tr{break-inside:avoid}.card,table,.notice{border-color:#999}}
</style>
</head>
<body>
<main>
<h1>Reporte de Mac Toolkit</h1>
<p class="sub">{{.GeneratedAt.Format "2006-01-02 15:04"}} · mac-toolkit {{.ToolVersion}}{{if eq .Variant "share"}} · variante compartible{{end}}</p>
{{if eq .Variant "share"}}<p class="notice">Variante compartible: no incluye rutas, nombres de archivo, etiquetas de elementos ni nombres de apps.</p>{{end}}

<h2>1. Equipo</h2>
<table><thead><tr><th>Campo</th><th>Valor</th></tr></thead><tbody>
{{range .HostRows}}<tr><td>{{index . 0}}</td><td>{{index . 1}}</td></tr>
{{end}}</tbody></table>

<h2>2. Resumen ejecutivo</h2>
<div class="cards">
<div class="card {{statusClass .Summary.Status}}"><div class="k">Estado general</div>
<div class="v {{statusClass .Summary.Status}}">{{statusLabel .Summary.Status}}</div></div>
<div class="card"><div class="k">Total recuperable</div><div class="v">{{bytes .Disk.Reclaimable.TotalBytes}}</div>
<div class="n">sin contar el uso del volumen</div></div>
<div class="card ok"><div class="k">Seguro</div><div class="v">{{bytes .Disk.Reclaimable.SafeBytes}}</div></div>
<div class="card warn"><div class="k">A revisar</div><div class="v">{{bytes .Disk.Reclaimable.ReviewBytes}}</div></div>
</div>
{{if .Summary.Top}}<table><thead><tr><th>#</th><th>Hallazgo</th><th>Severidad</th><th>Acción</th></tr></thead><tbody>
{{range $i, $f := .Summary.Top}}<tr><td>{{inc $i}}</td><td>{{$f.Title}}</td>
<td><span class="badge {{sevClass $f.Severity}}">{{sevLabel $f.Severity}}</span></td><td>{{$f.Action}}</td></tr>
{{end}}</tbody></table>{{else}}<p>Sin hallazgos.</p>{{end}}

<h2>3. Sistema</h2>
{{with .System.Memory}}<h3>Memoria</h3>
<table><thead><tr><th>Métrica</th><th>Valor</th></tr></thead><tbody>
<tr><td>Usada</td><td><span class="bar {{levelClass .UsedPct}}"><span style="width:{{pctw .UsedPct}}%"></span></span>
{{f0 .UsedPct}}% ({{f1 .UsedGB}} GB de {{f1 .TotalGB}} GB)</td></tr>
<tr><td>Presión</td><td>{{.Pressure}}</td></tr>
{{if .SwapLine}}<tr><td>Swap</td><td>{{.SwapLine}}</td></tr>{{end}}
<tr><td>Tráfico de swap</td><td>{{.SwapTrafficLine}}</td></tr>
<tr><td>Compresor</td><td>{{.CompressorLine}}</td></tr>
{{if .PagingVerdict}}<tr><td>Paginación</td><td>{{.PagingVerdict}}</td></tr>{{end}}
</tbody></table>{{end}}
{{with .System.CPU}}<h3>CPU</h3>
<table><thead><tr><th>Métrica</th><th>Valor</th></tr></thead><tbody>
<tr><td>Uso</td><td>usuario {{f1 .UserPct}}% · sistema {{f1 .SysPct}}% · libre {{f1 .IdlePct}}%</td></tr>
<tr><td>Carga (1, 5 y 15 min)</td><td>{{f2 .Load1}} · {{f2 .Load5}} · {{f2 .Load15}}</td></tr>
<tr><td>Carga por núcleo (5 min)</td><td>{{f2 .LoadPerCore}}</td></tr>
{{if .Verdict}}<tr><td>Veredicto</td><td>{{.Verdict}}</td></tr>{{end}}
</tbody></table>{{end}}
{{if .System.Thermal}}<p><strong>Térmico:</strong> {{.System.Thermal}}{{with .System.SpeedLimitPct}} (límite de velocidad de CPU: {{.}}%){{end}}</p>{{end}}

<h2>4. Disco</h2>
<h3>Uso del volumen (contexto, no se suma)</h3>
{{if .Disk.Volume.Known}}<table><thead><tr><th class="num">Total</th><th class="num">Usado</th><th class="num">Libre</th><th>Uso</th></tr></thead><tbody>
<tr><td class="num">{{bytes .Disk.Volume.TotalBytes}}</td><td class="num">{{bytes .Disk.Volume.UsedBytes}}</td>
<td class="num">{{bytes .Disk.Volume.FreeBytes}}</td>
<td><span class="bar {{levelClass (f64 .Disk.Volume.UsedPct)}}"><span style="width:{{pctw (f64 .Disk.Volume.UsedPct)}}%"></span></span> {{.Disk.Volume.UsedPct}}%</td></tr>
</tbody></table>{{else}}<p>No disponible.</p>{{end}}
<h3>Recuperable</h3>
<table><thead><tr><th>Concepto</th><th class="num">Tamaño</th></tr></thead><tbody>
<tr><td>Seguro (safe)</td><td class="num">{{bytes .Disk.Reclaimable.SafeBytes}}</td></tr>
<tr><td>A revisar (warn y danger)</td><td class="num">{{bytes .Disk.Reclaimable.ReviewBytes}}</td></tr>
<tr><td><strong>Total recuperable</strong></td><td class="num"><strong>{{bytes .Disk.Reclaimable.TotalBytes}}</strong></td></tr>
</tbody></table>
{{if .Disk.Domains}}<table><thead><tr><th>Dominio</th><th>Severidad</th><th class="num">Tamaño</th><th class="num">Elementos</th><th>Proporción</th></tr></thead><tbody>
{{range .Disk.Domains}}<tr><td>{{.Domain}}</td><td class="{{domainClass .Severity}}">{{.Severity}}</td>
<td class="num">{{bytes .SizeBytes}}</td><td class="num">{{.Items}}</td>
<td><span class="bar {{domainClass .Severity}}"><span style="width:{{pctw .SharePct}}%"></span></span> {{f0 .SharePct}}%</td></tr>
{{end}}</tbody></table>{{end}}
{{if .Disk.Failed}}<h3>Dominios con error</h3>
<table><thead><tr><th>Dominio</th><th>Error</th></tr></thead><tbody>
{{range .Disk.Failed}}<tr><td>{{.Domain}}</td><td>{{.Error}}</td></tr>
{{end}}</tbody></table>{{end}}

{{if .Details}}<h2>5. Detalle por dominio</h2>
{{range .Details}}<h3>{{.Domain}} · {{bytes .SizeBytes}}</h3>
<table><thead><tr><th class="num">Tamaño</th><th>Riesgo</th><th>Seguro</th><th>Elemento</th><th>Ruta</th></tr></thead><tbody>
{{range .Items}}<tr><td class="num">{{bytes .SizeBytes}}</td><td class="{{riskClass .Risk}}">{{.Risk}}</td>
<td>{{yesNo .Safe}}</td><td>{{.Label}}</td><td><code>{{short .Path}}</code></td></tr>
{{end}}</tbody></table>
{{if .More}}<p class="more">… y {{.More}} más ({{bytes .MoreBytes}})</p>{{end}}
{{end}}{{end}}

<h2>6. Procesos</h2>
<p class="sub">Apps agregadas por aplicación, sin PIDs.</p>
<h3>Top por memoria</h3>
{{template "apps" .Processes.TopMemory}}
<h3>Top por CPU</h3>
{{template "apps" .Processes.TopCPU}}

<h2>7. Recomendaciones</h2>
<p class="sub">Motor: {{.Recommendations.Engine}}. Son sugerencias: el toolkit no ejecuta ninguna.</p>
{{template "findings" (titled "Rendimiento" .Recommendations.Performance)}}
{{template "findings" (titled "Quick wins (seguros)" .Recommendations.QuickWins)}}
{{template "findings" (titled "A revisar" .Recommendations.Review)}}
<footer>Generado por mac-toolkit. Solo lectura del sistema.</footer>
</main>
</body>
</html>
{{define "apps"}}{{if .}}<table><thead><tr><th>#</th><th>App</th><th class="num">Memoria</th><th class="num">CPU</th><th class="num">Procesos</th></tr></thead><tbody>
{{range $i, $a := .}}<tr><td>{{inc $i}}</td><td>{{$a.Name}}</td><td class="num">{{f0 $a.MemMB}} MB</td>
<td class="num">{{f1 $a.CPUPct}}%</td><td class="num">{{$a.Procs}}</td></tr>
{{end}}</tbody></table>{{else}}<p>Sin datos.</p>{{end}}{{end}}
{{define "findings"}}{{if .List}}<h3>{{.Title}}</h3>
<table><thead><tr><th>Hallazgo</th><th>Severidad</th><th>Detalle</th><th>Acción</th></tr></thead><tbody>
{{range .List}}<tr><td>{{.Title}}</td><td><span class="badge {{sevClass .Severity}}">{{sevLabel .Severity}}</span></td>
<td>{{.Detail}}</td><td>{{.Action}}</td></tr>
{{end}}</tbody></table>{{end}}{{end}}
`
