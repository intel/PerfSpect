package report

// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bytes"
	"fmt"
	"html"
	htmltemplate "html/template"
	"log/slog"
	"perfspect/internal/table"
	"strings"
	texttemplate "text/template" // nosemgrep
)

// Package-level maps for custom HTML renderers
var customHTMLRenderers = map[string]table.HTMLTableRenderer{}

var customHTMLMultiTargetRenderers = map[string]table.HTMLMultiTargetTableRenderer{}

// getCustomHTMLRenderer returns the custom renderer for a table, or nil if no custom renderer exists
func getCustomHTMLRenderer(tableName string) table.HTMLTableRenderer {
	return customHTMLRenderers[tableName]
}

// getCustomHTMLMultiTargetRenderer returns the custom multi-target renderer for a table, or nil if no custom renderer exists
func getCustomHTMLMultiTargetRenderer(tableName string) table.HTMLMultiTargetTableRenderer {
	return customHTMLMultiTargetRenderers[tableName]
}

// RegisterHTMLRenderer allows external packages to register custom HTML renderers for specific tables
func RegisterHTMLRenderer(tableName string, renderer table.HTMLTableRenderer) {
	customHTMLRenderers[tableName] = renderer
}

// RegisterHTMLMultiTargetRenderer allows external packages to register custom multi-target HTML renderers for specific tables
func RegisterHTMLMultiTargetRenderer(tableName string, renderer table.HTMLMultiTargetTableRenderer) {
	customHTMLMultiTargetRenderers[tableName] = renderer
}

func getHtmlReportBegin() string {
	var sb strings.Builder
	sb.WriteString(`<!--
 * Copyright (C) 2024 Intel Corporation
 * SPDX-License-Identifier: MIT
-->
`)
	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
`)
	sb.WriteString("<head>\n")
	sb.WriteString(`    <meta charset="UTF-8">
    <title>Intel&reg; PerfSpect</title>
    <link rel="icon" type="image/x-icon" href="https://www.intel.com/favicon.ico">
    <meta name="viewport" content="width=device-width, initial-scale=1">
`)
	// link the style sheets and javascript
	sb.WriteString(`
	<link rel="stylesheet" href="https://unpkg.com/normalize.css@8.0.1/normalize.css" integrity="sha384-M86HUGbBFILBBZ9ykMAbT3nVb0+2C7yZlF8X2CiKNpDOQjKroMJqIeGZ/Le8N2Qp" crossorigin="anonymous" referrerpolicy="no-referrer" />
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/purecss@3.0.0/build/pure-min.css" integrity="sha384-X38yfunGUhNzHpBaEBsWLO+A0HDYOQi8ufWDkZ0k9e0eXz/tH3II7uKZ9msv++Ls" crossorigin="anonymous" referrerpolicy="no-referrer" />
    <script src="https://unpkg.com/chart.js@3.7.1/dist/chart.min.js" integrity="sha384-7NrRHqlWUj2hJl3a/dZj/a1GxuQc56mJ3aYsEnydBYrY1jR+RSt6SBvK3sHfj+mJ" crossorigin="anonymous"  referrerpolicy="no-referrer"></script>
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css">
	<link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/d3-flame-graph@4.1.3/dist/d3-flamegraph.css">
    <script type="text/javascript" src="https://d3js.org/d3.v7.js"></script>
    <script type="text/javascript" src="https://cdn.jsdelivr.net/npm/d3-flame-graph@4.1.3/dist/d3-flamegraph.min.js"></script>
	`)
	// add content class style
	sb.WriteString(`
	<style>
        .content {
            padding: 0 2em;
            line-height: 1.6em;
        }
        .content h2 {
            font-weight: 300;
            color: #888;
        }
        .content h2:before {
            content: '';
            display: block;
            position: relative;
            width: 0;
            height: 5em;
            margin-top: -5em
        }
	</style>
`)
	// add sidebar class styles
	sb.WriteString(`
	<style>
		.sidebar {
            height: 100%;
            width: 0;
            position: fixed;
            z-index: 1;
            top: 0;
            left: 0;
            background-color: #111;
            overflow-x: hidden;
            transition: 0.5s;
            padding-top: 30px;
            padding-left: 0px;
        }
        .sidebar h1 {
            position: absolute;
            top: 0;
            padding: 0px 8px 8px 35px;
            text-decoration: none;
            color: #fff;
            background-color: #1f8dd6;
            display: block;
            transition: 0.3s;
        }
		.sidebar h2 {
            padding: 8px 4px 2px 35px;
            text-decoration: none;
            color: #818181;
            display: block;
		}
        .sidebar a {
            padding: 0px 4px 2px 35px;
            text-decoration: none;
            color: #818181;
            display: block;
            transition: 0.3s;
        }
        .sidebar a:hover {
            color: #f1f1f1;
        }
        .sidebar .togglebtn {
            position: absolute;
            top: 0;
            right: 0px;
            font-size: 25px;
            padding-left: 5px;
            color: #ccc;
            background-color: #1f8dd6;
        }
        .sidebar .togglebtn:hover {
            color: #666;
        }
		.field-description {
			position: relative;
			display: inline-block;
			margin-left: 5px;
			cursor: help;
		}
		.field-description .tooltip-icon {
			color: #fff;
			font-size: 12px;
			border: 1px solid #2196F3;
			border-radius: 50%;
			width: 16px;
			height: 16px;
			text-align: center;
			line-height: 14px;
			background-color: #2196F3;
			transition: background-color 0.2s, border-color 0.2s;
		}
		.field-description:hover .tooltip-icon {
			background-color: #1976D2;
			border-color: #1976D2;
		}
		.field-description .tooltip-text {
			visibility: hidden;
			width: 250px;
			background-color: #333;
			color: #fff;
			text-align: left;
			border-radius: 6px;
			padding: 8px;
			position: absolute;
			z-index: 1000;
			bottom: 125%;
			left: 50%;
			margin-left: -125px;
			opacity: 0;
			transition: opacity 0.3s;
			font-size: 12px;
			box-shadow: 0px 0px 6px rgba(0,0,0,0.2);
		}
		.field-description .tooltip-text::after {
			content: "";
			position: absolute;
			top: 100%;
			left: 50%;
			margin-left: -5px;
			border-width: 5px;
			border-style: solid;
			border-color: #333 transparent transparent transparent;
		}
		.field-description:hover .tooltip-text {
			visibility: visible;
			opacity: 1;
		}
	</style>
	`)
	sb.WriteString("</head>\n")

	return sb.String()
}

func getHtmlReportMenu(allTableValues []table.TableValues) string {
	var sb strings.Builder
	// if none of the tables have menu labels, don't add the sidebar
	hasMenuLabels := false
	for _, tableValues := range allTableValues {
		if tableValues.MenuLabel != "" {
			hasMenuLabels = true
			break
		}
	}
	if hasMenuLabels {
		sb.WriteString("<div id=\"mySidebar\" class=\"sidebar\">\n")
		sb.WriteString("<a href=\"#\" style=\"position: absolute;top: 0; padding-left: 7px; padding-right: 117px; color: #fff; background-color: #1f8dd6\">CONTENTS</a>\n")
		sb.WriteString("<a href=\"javascript:void(0)\" class=\"togglebtn\" onclick=\"toggleNav()\">&lt;</a>\n")
		// insert menu items into sidebar
		for _, tableValues := range allTableValues {
			if tableValues.MenuLabel != "" {
				sb.WriteString(fmt.Sprintf("<a href=\"#%s\">%s</a>\n", html.EscapeString(tableValues.Name), html.EscapeString(tableValues.MenuLabel)))
			}
		}
		sb.WriteString("</div>\n") // end of sidebar
	}
	return sb.String()
}

func getHtmlReportSidebarJavascript() string {
	return `
	<script>
		const widthOpen="225px"
		const widthClosed="30px"
		function openNav() {
			document.getElementById("mySidebar").style.width = widthOpen;
			document.getElementById("myTables").style.marginLeft = widthOpen;
			document.querySelector(".togglebtn").innerHTML="<"
		}
		function closeNav() {
			document.getElementById("mySidebar").style.width = widthClosed;
			document.getElementById("myTables").style.marginLeft= widthClosed;
			document.querySelector(".togglebtn").innerHTML=">"
		}
		function toggleNav() {
			if (document.getElementById("mySidebar").style.width !== widthOpen) {
				openNav()
			} else {
				closeNav()
			}
		}
		// open on startup
		openNav()
	</script>
	`
}

func createHtmlReport(allTableValues []table.TableValues, targetName string) (out []byte, err error) {
	var sb strings.Builder
	sb.WriteString(getHtmlReportBegin())

	// body starts here
	sb.WriteString("<body>\n")
	sb.WriteString("<main class=\"content\">\n")
	// add the sidebar/menu
	sb.WriteString(getHtmlReportMenu(allTableValues))
	// add the tables
	sb.WriteString("<div id=\"myTables\">\n")
	sb.WriteString("<h1>Intel&reg; PerfSpect</h1>\n")
	sb.WriteString(`                    
<noscript>
	<h3>JavaScript is disabled. Functionality is limited.</h3>
</noscript>
`)
	for _, tableValues := range allTableValues {
		// print the table name
		sb.WriteString(fmt.Sprintf("<h2 id=\"%[1]s\">%[1]s</h2>\n", html.EscapeString(tableValues.Name)))
		// if there's no data in the table, print a message and continue
		if len(tableValues.Fields) == 0 || len(tableValues.Fields[0].Values) == 0 {
			msg := NoDataFound
			if tableValues.NoDataFound != "" {
				msg = tableValues.NoDataFound
			}
			sb.WriteString("<p>" + msg + "</p>\n")
			continue
		}
		// render the tables
		if renderer := getCustomHTMLRenderer(tableValues.Name); renderer != nil { // custom table renderer
			sb.WriteString(renderer(tableValues, targetName))
		} else {
			sb.WriteString(DefaultHTMLTableRendererFunc(tableValues))
		}
	}
	sb.WriteString("</div>\n") // end of myTables
	sb.WriteString("</main>\n")

	// add the sidebar toggle function
	sb.WriteString(getHtmlReportSidebarJavascript())

	sb.WriteString("</body>\n")
	sb.WriteString("</html>\n")
	out = []byte(sb.String())
	return
}

func createHtmlReportMultiTarget(allTargetsTableValues [][]table.TableValues, targetNames []string, allTableNames []string) (out []byte, err error) {
	if len(allTargetsTableValues) == 0 {
		return nil, fmt.Errorf("no target table values provided")
	}
	var sb strings.Builder
	sb.WriteString(getHtmlReportBegin())

	// body starts here
	sb.WriteString("<body>\n")
	sb.WriteString("<main class=\"content\">\n")
	// add the sidebar/menu
	sb.WriteString(getHtmlReportMenu(allTargetsTableValues[0]))
	// add the tables
	sb.WriteString("<div id=\"myTables\">\n")
	sb.WriteString("<h1>Intel&reg; PerfSpect</h1>\n")
	sb.WriteString(`                    
<noscript>
	<h3>JavaScript is disabled. Functionality is limited.</h3>
</noscript>
`)
	// print the tables in the order they were passed in
	for _, tableName := range allTableNames {
		oneTableValuesForAllTargets := []table.TableValues{}
		// build list of target names and table.TableValues for targets that have values for this table
		tableTargets := []string{}
		tableValues := []table.TableValues{}
		for targetIndex, targetTableValues := range allTargetsTableValues {
			tableIndex := findTableIndex(targetTableValues, tableName)
			if tableIndex == -1 {
				continue
			}
			tableTargets = append(tableTargets, targetNames[targetIndex])
			tableValues = append(tableValues, targetTableValues[tableIndex])
		}
		// loop through the targets that have values for this table
		for targetIndex, targetTableValues := range tableValues {
			targetName := tableTargets[targetIndex]
			// if the table has rows and no custom renderer, print the table for the target normally
			if targetTableValues.HasRows && getCustomHTMLMultiTargetRenderer(targetTableValues.Name) == nil {
				// print the table name only one time per table
				if targetIndex == 0 {
					sb.WriteString(fmt.Sprintf("<h2 id=\"%[1]s\">%[1]s</h2>\n", html.EscapeString(targetTableValues.Name)))
				}
				// print the target name
				sb.WriteString(fmt.Sprintf("<h3>%s</h3>\n", targetName))
				// if there's no data in the table, print a message and continue
				if len(targetTableValues.Fields) == 0 || len(targetTableValues.Fields[0].Values) == 0 {
					sb.WriteString("<p>" + NoDataFound + "</p>\n")
					continue
				}
				if renderer := getCustomHTMLRenderer(targetTableValues.Name); renderer != nil { // custom table renderer
					sb.WriteString(renderer(targetTableValues, targetNames[targetIndex]))
				} else {
					sb.WriteString(DefaultHTMLTableRendererFunc(targetTableValues))
				}
			} else { // if the table has no rows or a custom renderer, add the table to the list to render as a multi-target table
				oneTableValuesForAllTargets = append(oneTableValuesForAllTargets, targetTableValues)
			}
		}
		// print the multi-target table, if any
		if len(oneTableValuesForAllTargets) > 0 {
			sb.WriteString(fmt.Sprintf("<h2 id=\"%[1]s\">%[1]s</h2>\n", html.EscapeString(oneTableValuesForAllTargets[0].Name)))
			if renderer := getCustomHTMLMultiTargetRenderer(oneTableValuesForAllTargets[0].Name); renderer != nil {
				sb.WriteString(renderer(oneTableValuesForAllTargets, targetNames))
			} else {
				// render the multi-target table
				sb.WriteString(RenderMultiTargetTableValuesAsHTML(oneTableValuesForAllTargets, targetNames))
			}
		}
	}
	sb.WriteString("</div>\n") // end of myTables
	sb.WriteString("</main>\n")

	// add the sidebar toggle function
	sb.WriteString(getHtmlReportSidebarJavascript())

	sb.WriteString("</body>\n")
	sb.WriteString("</html>\n")
	out = []byte(sb.String())
	return
}

// findTableIndex
func findTableIndex(tableValues []table.TableValues, tableName string) int {
	for i, tableValue := range tableValues {
		if tableValue.Name == tableName {
			return i
		}
	}
	return -1
}

const datasetTemplate = `
{
	label: '{{.Label}}',
	data: [{{.Data}}],
	backgroundColor: '{{.Color}}',
	borderColor: '{{.Color}}',
	borderWidth: 1,
	showLine: true,
	hidden: {{.Hidden}}
}
`
const lineChartTemplate = `<div class="chart-container" style="max-width: 900px">
<canvas id="{{.ID}}"></canvas>
</div>
<script>
new Chart(document.getElementById('{{.ID}}'), {
    type: 'line',
    data: {
		labels: [{{.Labels}}],
        datasets: [{{.Datasets}}]
    },
    options: {
        aspectRatio: {{.AspectRatio}},
        scales: {
            x: {
                beginAtZero: false,
                title: {
                    text: "{{.XaxisText}}",
                    display: true
                },
				ticks: {
					maxRotation: 90,
					minRotation: 45
                }
            },
            y: {
                title: {
                    text: "{{.YaxisText}}",
                    display: true
                },
				suggestedMin: {{.SuggestedMin}},
				suggestedMax: {{.SuggestedMax}},
            }
        },
        plugins: {
            title: {
                text: "{{.TitleText}}",
                display: {{.DisplayTitle}},
                font: {
                    size: 18
                }
            },
            tooltip: {
                callbacks: {
                    label: function(ctx) {
                        return ctx.dataset.label + " (" + ctx.parsed.x + ", " + ctx.parsed.y + ")";
                    }
                }
            },
            legend: {
                display: {{.DisplayLegend}}
            }
        }
    }
});
</script>
`
const scatterChartTemplate = `<div class="chart-container" style="max-width: 900px">
<canvas id="{{.ID}}"></canvas>
</div>
<script>
new Chart(document.getElementById('{{.ID}}'), {
    type: 'scatter',
    data: {
        datasets: [{{.Datasets}}]
    },
    options: {
        aspectRatio: {{.AspectRatio}},
        scales: {
            x: {
                beginAtZero: false,
                title: {
                    text: "{{.XaxisText}}",
                    display: true
                }
            },
            y: {
                title: {
                    text: "{{.YaxisText}}",
                    display: true
                },
				suggestedMin: {{.SuggestedMin}},
				suggestedMax: {{.SuggestedMax}},
            }
        },
        plugins: {
            title: {
                text: "{{.TitleText}}",
                display: {{.DisplayTitle}},
                font: {
                    size: 18
                }
            },
            tooltip: {
                callbacks: {
                    label: function(ctx) {
                        return ctx.dataset.label + " (" + ctx.parsed.x + ", " + ctx.parsed.y + ")";
                    }
                }
            },
            legend: {
                display: {{.DisplayLegend}}
            }
        }
    }
});
</script>
`

type ChartTemplateStruct struct {
	ID            string
	Labels        string // only for line charts
	Datasets      string
	XaxisText     string
	YaxisText     string
	TitleText     string
	DisplayTitle  string
	DisplayLegend string
	AspectRatio   string
	SuggestedMin  string
	SuggestedMax  string
}

// CreateFieldNameWithDescription creates HTML for a field name with optional description tooltip
func CreateFieldNameWithDescription(fieldName, description string) string {
	if description == "" {
		return htmltemplate.HTMLEscapeString(fieldName)
	}
	return htmltemplate.HTMLEscapeString(fieldName) + `<span class="field-description"><span class="tooltip-icon">?</span><span class="tooltip-text">` + htmltemplate.HTMLEscapeString(description) + `</span></span>`
}

func RenderHTMLTable(tableHeaders []string, tableValues [][]string, class string, valuesStyle [][]string) string {
	return renderHTMLTableWithDescriptions(tableHeaders, nil, tableValues, class, valuesStyle)
}

// renderHTMLTableWithDescriptions renders an HTML table with optional header descriptions
func renderHTMLTableWithDescriptions(tableHeaders []string, headerDescriptions []string, tableValues [][]string, class string, valuesStyle [][]string) string {
	var sb strings.Builder
	sb.WriteString(`<table class="` + class + `">`)
	if len(tableHeaders) > 0 {
		sb.WriteString(`<thead>`)
		sb.WriteString(`<tr>`)
		for i, label := range tableHeaders {
			var description string
			if headerDescriptions != nil && i < len(headerDescriptions) {
				description = headerDescriptions[i]
			}
			sb.WriteString(`<th>` + CreateFieldNameWithDescription(label, description) + `</th>`)
		}
		sb.WriteString(`</tr>`)
		sb.WriteString(`</thead>`)
	}
	sb.WriteString(`<tbody>`)
	for rowIdx, rowValues := range tableValues {
		sb.WriteString(`<tr>`)
		for colIdx, value := range rowValues {
			var style string
			if len(valuesStyle) > rowIdx && len(valuesStyle[rowIdx]) > colIdx {
				style = ` style="` + valuesStyle[rowIdx][colIdx] + `"`
			}
			sb.WriteString(`<td` + style + `>` + value + `</td>`)
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody>`)
	sb.WriteString(`</table>`)
	return sb.String()
}

func DefaultHTMLTableRendererFunc(tableValues table.TableValues) string {
	if tableValues.HasRows { // print the field names as column headings across the top of the table
		headers := []string{}
		headerDescriptions := []string{}
		for _, field := range tableValues.Fields {
			headers = append(headers, field.Name)
			headerDescriptions = append(headerDescriptions, field.Description)
		}
		values := [][]string{}
		for row := range tableValues.Fields[0].Values {
			rowValues := []string{}
			for _, field := range tableValues.Fields {
				rowValues = append(rowValues, htmltemplate.HTMLEscapeString(field.Values[row]))
			}
			values = append(values, rowValues)
		}
		return renderHTMLTableWithDescriptions(headers, headerDescriptions, values, "pure-table pure-table-striped", [][]string{})
	} else { // print the field name followed by its value
		values := [][]string{}
		var tableValueStyles [][]string
		for _, field := range tableValues.Fields {
			rowValues := []string{}
			rowValues = append(rowValues, CreateFieldNameWithDescription(field.Name, field.Description))
			if len(field.Values) > 0 {
				rowValues = append(rowValues, htmltemplate.HTMLEscapeString(field.Values[0]))
			} else {
				rowValues = append(rowValues, "")
			}
			values = append(values, rowValues)
			tableValueStyles = append(tableValueStyles, []string{"font-weight:bold"})
		}
		return RenderHTMLTable([]string{}, values, "pure-table pure-table-striped", tableValueStyles)
	}
}

// RenderMultiTargetTableValuesAsHTML renders a table for multiple targets
// tableValues is a slice of table.TableValues, each of which represents the same table from a single target
func RenderMultiTargetTableValuesAsHTML(tableValues []table.TableValues, targetNames []string) string {
	if len(tableValues) == 0 {
		return ""
	}
	values := [][]string{}
	var tableValueStyles [][]string
	for fieldIndex, field := range tableValues[0].Fields {
		rowValues := []string{}
		rowValues = append(rowValues, CreateFieldNameWithDescription(field.Name, field.Description))
		for _, targetTableValues := range tableValues {
			if len(targetTableValues.Fields) > fieldIndex && len(targetTableValues.Fields[fieldIndex].Values) > 0 {
				rowValues = append(rowValues, htmltemplate.HTMLEscapeString(targetTableValues.Fields[fieldIndex].Values[0]))
			} else {
				rowValues = append(rowValues, "")
			}
		}
		values = append(values, rowValues)
		tableValueStyles = append(tableValueStyles, []string{"font-weight:bold"})
	}
	headers := []string{""}
	headers = append(headers, targetNames...)
	return RenderHTMLTable(headers, values, "pure-table pure-table-striped", tableValueStyles)
}

// RenderChart generates an HTML/JavaScript representation of a chart using the provided data and configuration.
// It supports different chart types (e.g., "line", "scatter") and uses Go templates to format the datasets and chart.
// Parameters:
//   - chartType: the type of chart to render ("line", "scatter").
//   - allFormattedPoints: a slice of strings, each representing formatted data points for a dataset.
//   - datasetNames: a slice of dataset names corresponding to each dataset.
//   - xAxisLabels: a slice of labels for the x-axis (used for line charts).
//   - config: a chartTemplateStruct containing chart configuration and template variables.
//   - datasetHiddenFlags: a slice of booleans indicating whether each dataset should be hidden initially.
//
// Returns:
//   - A string containing the rendered chart HTML/JavaScript, or an error message if rendering fails.
func RenderChart(chartType string, allFormattedPoints []string, datasetNames []string, xAxisLabels []string, config ChartTemplateStruct, datasetHiddenFlags []bool) string {
	datasets := []string{}
	for dataIdx, formattedPoints := range allFormattedPoints {
		specValues := formattedPoints
		dst := texttemplate.Must(texttemplate.New("datasetTemplate").Parse(datasetTemplate))
		buf := new(bytes.Buffer)
		// determine hidden flag for this dataset
		hidden := "false"
		if datasetHiddenFlags != nil && dataIdx < len(datasetHiddenFlags) && datasetHiddenFlags[dataIdx] {
			hidden = "true"
		}
		err := dst.Execute(buf, struct {
			Label  string
			Data   string
			Color  string
			Hidden string
		}{
			Label:  datasetNames[dataIdx],
			Data:   specValues,
			Color:  getColor(dataIdx),
			Hidden: hidden,
		})
		if err != nil {
			slog.Error("error executing template", slog.String("error", err.Error()))
			return "Error rendering chart."
		}
		datasets = append(datasets, buf.String())
	}
	var chartTemplate string
	switch chartType {
	case "line":
		chartTemplate = lineChartTemplate
	case "scatter":
		chartTemplate = scatterChartTemplate
	default:
		panic("unknown chart type")
	}
	sct := texttemplate.Must(texttemplate.New("chartTemplate").Parse(chartTemplate))
	buf := new(bytes.Buffer)
	config.Datasets = strings.Join(datasets, ",")
	if chartType == "line" {
		config.Labels = func() string {
			var labels []string
			for _, label := range xAxisLabels {
				labels = append(labels, fmt.Sprintf("'%s'", label))
			}
			return strings.Join(labels, ",")
		}()
	}
	err := sct.Execute(buf, config)
	if err != nil {
		slog.Error("error executing template", slog.String("error", err.Error()))
		return "Error rendering chart."
	}
	out := buf.String()
	out += "\n"
	return out
}

type ScatterPoint struct {
	X float64
	Y float64
}

// RenderScatterChart generates an HTML string for a scatter chart using the provided data and configuration.
//
// Parameters:
//
//	data   - 2D slice of scatterPoint values, where each inner slice represents a dataset's data points.
//	datasetNames - Slice of strings representing the names of each dataset.
//	config - chartTemplateStruct containing chart configuration options.
//
// Returns:
//
//	A string containing the rendered HTML for the scatter chart.
func RenderScatterChart(data [][]ScatterPoint, datasetNames []string, config ChartTemplateStruct) string {
	allFormattedPoints := []string{}
	for dataIdx := range data {
		formattedPoints := []string{}
		for _, point := range data[dataIdx] {
			formattedPoints = append(formattedPoints, fmt.Sprintf("{x: %f, y: %f}", point.X, point.Y))
		}
		allFormattedPoints = append(allFormattedPoints, strings.Join(formattedPoints, ","))
	}
	return RenderChart("scatter", allFormattedPoints, datasetNames, nil, config, nil)
}

func getColor(idx int) string {
	// color-blind safe palette from here: http://mkweb.bcgsc.ca/colorblind/palettes.mhtml#page-container
	colors := []string{"#9F0162", "#009F81", "#FF5AAF", "#00FCCF", "#8400CD", "#008DF9", "#00C2F9", "#FFB2FD", "#A40122", "#E20134", "#FF6E3A", "#FFC33B"}
	return colors[idx%len(colors)]
}
