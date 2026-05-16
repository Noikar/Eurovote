package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func cleanText(s string) string { return strings.TrimSpace(s) }

func parseRunOrderFromRows(rows *goquery.Selection) map[string]int {
	order := make(map[string]int)
	rows.Each(func(_ int, row *goquery.Selection) {
		if row.Find("th[scope=col]").Length() > 0 { return }
		th := cleanText(row.Find("th[scope=row]").First().Text())
		n, err := strconv.Atoi(th)
		if err != nil || n < 1 { return }
		country := cleanText(row.Find("td").First().Text())
		if country != "" && order[country] == 0 {
			order[country] = n
		}
	})
	return order
}

func main() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://en.wikipedia.org/wiki/Eurovision_Song_Contest_2026", nil)
	req.Header.Set("User-Agent", "Eurovote/1.0")
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	doc, _ := goquery.NewDocumentFromReader(resp.Body)

	var rows *goquery.Selection
	doc.Find("h2, h3, .mw-heading").EachWithBreak(func(_ int, h *goquery.Selection) bool {
		text := strings.ToLower(cleanText(h.Text()))
		if !strings.Contains(text, "final") || strings.Contains(text, "semi") { return true }
		anchor := h
		if h.Parent().HasClass("mw-heading") { anchor = h.Parent() }
		anchor.NextUntil("h2, h3, .mw-heading").Each(func(_ int, s *goquery.Selection) {
			var found *goquery.Selection
			if goquery.NodeName(s) == "table" {
				found = s.Find("tr")
			} else {
				found = s.Find("table.wikitable tr")
			}
			if rows == nil { rows = found } else { rows = rows.AddSelection(found) }
		})
		return false
	})

	order := parseRunOrderFromRows(rows)
	fmt.Printf("Got %d countries\n", len(order))

	type entry struct{ country string; n int }
	var sorted []entry
	for c, n := range order { sorted = append(sorted, entry{c, n}) }
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].n < sorted[j].n })
	for _, e := range sorted { fmt.Printf("  %d. %s\n", e.n, e.country) }
}
