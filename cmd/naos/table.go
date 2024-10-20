package main

import (
	"bytes"
	"fmt"
	"sort"
)

type table struct {
	data [][]string

	writtenLines int
}

func newTable(headers ...string) *table {
	return &table{data: [][]string{headers}}
}

func (t *table) add(cells ...string) *table {
	t.data = append(t.data, cells)
	return t
}

func (t *table) string() string {
	// prepare buffer
	buf := new(bytes.Buffer)

	// prepare max function
	max := func(p *int, v int) {
		if *p < v {
			*p = v
		}
	}

	// prepare lengths
	lengths := make([]int, len(t.data[0]))

	// get max cell lengths
	for _, v := range t.data {
		for i, cell := range v {
			max(&lengths[i], len(cell))
		}
	}

	// construct string
	for _, v := range t.data {
		buf.WriteString(makeRow(v, lengths))
		buf.WriteString("\n")
	}

	return buf.String()
}

func (t *table) show(sortColumn int) {
	// get a sub slice for sorting (without) header
	sub := t.data[1:]

	// sort table using a column
	if sortColumn >= 0 {
		sort.Slice(sub, func(i, j int) bool {
			return sub[i][sortColumn] < sub[j][sortColumn]
		})
	}

	// construct full slice
	full := append([][]string{}, t.data[0])
	full = append(full, sub...)
	t.data = full

	// show table
	fmt.Print(t.string())

	// save written lines
	t.writtenLines = len(t.data)
}

func (t *table) clear() {
	// move cursor up the amount of written lines
	if t.writtenLines > 0 {
		fmt.Printf("\033[%dA", t.writtenLines)
	}

	// reset data, but retain headers
	t.data = [][]string{t.data[0]}
}

func makeRow(cells []string, lengths []int) string {
	// prepare buffer
	buf := new(bytes.Buffer)

	// iterate over all cells
	for i, cell := range cells {
		// write content
		buf.WriteString(cell)

		// fill to right
		if i < len(cells)-1 {
			buf.Write(bytes.Repeat([]byte(" "), lengths[i]-len(cell)))
		}

		// add padding
		buf.WriteString("   ")
	}

	return buf.String()
}
