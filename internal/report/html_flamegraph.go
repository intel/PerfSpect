package report

// Copyright (C) 2021-2024 Intel Corporation
// SPDX-License-Identifier: BSD-3-Clause

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2" // nosemgrep
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
func reverse(strings []string) {
	for i, j := 0, len(strings)-1; i < j; i, j = i+1, j-1 {
		strings[i], strings[j] = strings[j], strings[i]
	}
}

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

var maxStackDepth = 50

func convertFoldedToJSON(folded string) (out string, err error) {
	rootNode := Node{Name: "root", Value: 0, Children: make(map[string]*Node)}
	scanner := bufio.NewScanner(strings.NewReader(folded))
	for scanner.Scan() {
		line := scanner.Text()
		sep := strings.LastIndex(line, " ")
		s := line[:sep]
		v := line[sep+1:]
		stack := strings.Split(s, ";")
		reverse(stack)
		if len(stack) > maxStackDepth {
			log.Printf("Trimming call stack depth from %d to %d", len(stack), maxStackDepth)
			stack = stack[:maxStackDepth]
		}
		var i int
		i, err = strconv.Atoi(v)
		if err != nil {
			return
		}
		rootNode.Add(&stack, len(stack)-1, i)
	}
	outbytes, err := rootNode.MarshalJSON()
	out = string(outbytes)
	return
}

func renderFlameGraph(header string, tableValues TableValues, field string) (out string) {
	fieldIdx, err := getFieldIndex(field, tableValues)
	if err != nil {
		log.Panicf("didn't find expected field (%s) in table: %v", field, err)
	}
	folded := tableValues.Fields[fieldIdx].Values[0]
	if folded == "" {
		out += `<div class="fgheader clearfix"><h3 class="text-muted">` + header + `</h3></div>`
		msg := noDataFound
		if tableValues.NoDataFound != "" {
			msg = tableValues.NoDataFound
		}
		out += msg
		return
	}
	jsonStacks, err := convertFoldedToJSON(folded)
	if err != nil {
		log.Printf("failed to convert folded data: %v", err)
		out += "Error."
		return
	}
	fg := texttemplate.Must(texttemplate.New("flameGraphTemplate").Parse(flameGraphTemplate))
	buf := new(bytes.Buffer)
	err = fg.Execute(buf, flameGraphTemplateStruct{
		ID:     fmt.Sprintf("%d%s", rand.IntN(10000), header),
		Data:   jsonStacks,
		Header: header,
	})
	if err != nil {
		log.Printf("failed to render flame graph template: %v", err)
		out += "Error."
		return
	}
	out += buf.String()
	out += "\n"
	return
}
