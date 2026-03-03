// Copyright (C) 2021-2025 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

package flamegraph

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"perfspect/internal/report"
	"perfspect/internal/table"
	"perfspect/internal/util"
	"slices"
	"strconv"
	"strings"
	texttemplate "text/template" // nosemgrep
)

const flameGraphTemplate = `
<div class="fgcontainer">
	<div class="fgheader clearfix">
		<nav>
			<div class="pull-right">
			<form class="form-inline" id="form{{.ID}}">
				<a class="btn" href="javascript: resetZoom{{.ID}}();">Reset zoom</a>
				<a class="btn" href="javascript: clear{{.ID}}();">Clear</a>
				<div class="form-group">
				<input type="text" class="form-control" id="term{{.ID}}">
				</div>
				<a class="btn btn-primary" href="javascript: search{{.ID}}();">Search</a>
			</form>
			</div>
		</nav>
        <h3 class="text-muted">{{.Header}}</h3>
	</div>
	<div id="chart{{.ID}}"></div>
	<hr>
	<div id="details{{.ID}}"></div>
</div>
<script type="text/javascript">
  var chart{{.ID}} = flamegraph()
    .width(990)
	.cellHeight(18)
    .inverted(false)
	.sort(true)
	.minFrameSize(5);
  d3.select("#chart{{.ID}}")
    .datum({{.Data}})
    .call(chart{{.ID}});
  
    var details{{.ID}} = document.getElementById("details{{.ID}}");
    chart{{.ID}}.setDetailsElement(details{{.ID}});

    document.getElementById("form{{.ID}}").addEventListener("submit", function(event){
      event.preventDefault();
      search{{.ID}}();
    });

    function search{{.ID}}() {
      var term = document.getElementById("term{{.ID}}").value;
      chart{{.ID}}.search(term);
    }

    function clear{{.ID}}() {
      document.getElementById('term{{.ID}}').value = '';
      chart{{.ID}}.clear();
      chart{{.ID}}.search();
    }

    function resetZoom{{.ID}}() {
      chart{{.ID}}.resetZoom();
    }
</script>
`

type flameGraphTemplateStruct struct {
	ID     string
	Data   string
	Header string
}

// Folded data conversion adapted from https://github.com/spiermar/burn
// Copyright © 2017 Martin Spier <spiermar@gmail.com>
// Apache License, Version 2.0

type Node struct {
	Name     string
	Value    int
	Children map[string]*Node
}

func (n *Node) Add(stackPtr *[]string, index int, value int) {
	n.Value += value
	if index >= 0 {
		head := (*stackPtr)[index]
		childPtr, ok := n.Children[head]
		if !ok {
			childPtr = &(Node{head, 0, make(map[string]*Node)})
			n.Children[head] = childPtr
		}
		childPtr.Add(stackPtr, index-1, value)
	}
}

func (n *Node) MarshalJSON() ([]byte, error) {
	v := make([]Node, 0, len(n.Children))
	for _, value := range n.Children {
		v = append(v, *value)
	}

	return json.Marshal(&struct {
		Name     string `json:"name"`
		Value    int    `json:"value"`
		Children []Node `json:"children"`
	}{
		Name:     n.Name,
		Value:    n.Value,
		Children: v,
	})
}

func convertFoldedToJSON(folded string, maxStackDepth int) (out string, err error) {
	rootNode := Node{Name: "root", Value: 0, Children: make(map[string]*Node)}
	scanner := bufio.NewScanner(strings.NewReader(folded))
	for scanner.Scan() {
		line := scanner.Text()
		sep := strings.LastIndex(line, " ") // space separates the call stack from the count
		callstack := line[:sep]
		count := line[sep+1:]
		stack := strings.Split(callstack, ";") // semicolon separates the functions in the call stack
		slices.Reverse(stack)
		if maxStackDepth > 0 {
			if len(stack) > maxStackDepth {
				slog.Info("Trimming call stack depth", slog.Int("stack length", len(stack)), slog.Int("max depth", maxStackDepth))
				stack = stack[:maxStackDepth]
			}
		}
		var i int
		i, err = strconv.Atoi(count)
		if err != nil {
			return
		}
		rootNode.Add(&stack, len(stack)-1, i)
	}
	outbytes, err := rootNode.MarshalJSON()
	out = string(outbytes)
	return
}

func renderFlameGraph(header string, tableValues table.TableValues, field string) (out string) {
	maxDepthFieldIndex, err := table.GetFieldIndex("Maximum Render Depth", tableValues)
	if err != nil {
		slog.Error("didn't find expected field (Maximum Render Depth) in table", slog.String("error", err.Error()))
		return
	}
	maxDepth := tableValues.Fields[maxDepthFieldIndex].Values[0]
	if maxDepth == "" {
		slog.Error("maximum render depth field is empty")
		return
	}
	maxStackDepth, err := strconv.Atoi(maxDepth)
	if err != nil {
		slog.Error("failed to convert maximum stack depth", slog.String("error", err.Error()))
		return
	}
	fieldIdx, err := table.GetFieldIndex(field, tableValues)
	if err != nil {
		slog.Error("didn't find expected field in table", slog.String("field", field), slog.String("error", err.Error()))
		return
	}
	folded := tableValues.Fields[fieldIdx].Values[0]
	if folded == "" {
		out += `<div class="fgheader clearfix"><h3 class="text-muted">` + header + `</h3></div>`
		msg := report.NoDataFound
		if tableValues.NoDataFound != "" {
			msg = tableValues.NoDataFound
		}
		out += msg
		return
	}
	jsonStacks, err := convertFoldedToJSON(folded, maxStackDepth)
	if err != nil {
		slog.Error("failed to convert folded data", slog.String("error", err.Error()))
		out = ""
		return
	}
	fg := texttemplate.Must(texttemplate.New("flameGraphTemplate").Parse(flameGraphTemplate))
	buf := new(bytes.Buffer)
	err = fg.Execute(buf, flameGraphTemplateStruct{
		ID:     fmt.Sprintf("%d%s", util.RandUint(10000), strings.Split(header, " ")[0]),
		Data:   jsonStacks,
		Header: header,
	})
	if err != nil {
		slog.Error("failed to render flame graph template", slog.String("error", err.Error()))
		out = ""
		return
	}
	out += buf.String()
	out += "\n"
	return
}

func callStackFrequencyTableHTMLRenderer(tableValues table.TableValues, targetName string) string {
	out := `<style>

/* Custom page header */
.fgheader {
	padding-bottom: 15px;
	padding-right: 15px;
	padding-left: 15px;
	border-bottom: 1px solid #e5e5e5;
}

/* Make the masthead heading the same height as the navigation */
.fgheader h3 {
    margin-top: 0;
    margin-bottom: 0;
    line-height: 40px;
}

/* Customize container */
.fgcontainer {
	max-width: 990px;
}
</style>
`
	// get the perf event from the table values
	perfEventFieldIndex, err := table.GetFieldIndex("Perf Event", tableValues)
	if err != nil {
		slog.Error("didn't find expected field (Perf Event) in table", slog.String("error", err.Error()))
		return out
	}
	if len(tableValues.Fields[perfEventFieldIndex].Values) == 0 {
		slog.Error("no values for perf event field in table")
		return out
	}
	perfEvent := tableValues.Fields[perfEventFieldIndex].Values[0]
	out += renderFlameGraph(fmt.Sprintf("Native (perf record -e %s)", perfEvent), tableValues, "Native Stacks")

	// get the asprof arguments from the table values
	asprofArgumentsFieldIndex, err := table.GetFieldIndex("Asprof Arguments", tableValues)
	if err != nil {
		slog.Error("didn't find expected field (Asprof Arguments) in table", slog.String("error", err.Error()))
		return out
	}
	if len(tableValues.Fields[asprofArgumentsFieldIndex].Values) == 0 {
		slog.Error("no values for asprof arguments field in table")
		return out
	}
	asprofArguments := tableValues.Fields[asprofArgumentsFieldIndex].Values[0]
	if asprofArguments != "" {
		asprofArguments = " " + asprofArguments
	}
	out += renderFlameGraph(fmt.Sprintf("Java (asprof start%s)", asprofArguments), tableValues, "Java Stacks")
	return out
}
