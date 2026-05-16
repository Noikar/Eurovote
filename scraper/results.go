package scraper

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"eurovote/models"

	"github.com/PuerkitoBio/goquery"
)

// FetchSemiQualifiers fetches the countries that qualified from the given semi-final.
// Returns an error (including a user-friendly message) if results aren't available yet.
func FetchSemiQualifiers(year, semiNum int) ([]string, error) {
	ordinal := "First"
	if semiNum == 2 {
		ordinal = "Second"
	}
	yearStr := strconv.Itoa(year)

	// Try dedicated semi-final article pages first.
	urls := []string{
		"https://en.wikipedia.org/wiki/" + ordinal + "_semi-final_of_the_Eurovision_Song_Contest_" + yearStr,
		"https://en.wikipedia.org/wiki/Eurovision_Song_Contest_" + yearStr + "_%E2%80%93_" + ordinal + "_semi-final",
	}
	for _, u := range urls {
		doc, err := fetchDocURL(u)
		if err != nil {
			continue
		}
		if qs := parseQualifiers(doc); len(qs) > 0 {
			return qs, nil
		}
	}

	// Fall back to the main contest article and parse the SF-specific section.
	doc, err := fetchDoc(year)
	if err != nil {
		return nil, fmt.Errorf("results not yet available — %w", err)
	}
	qs := parseQualifiersFromSection(doc, semiNum)
	if len(qs) == 0 {
		return nil, fmt.Errorf("results not yet available on Wikipedia")
	}
	return qs, nil
}

// FetchFinalRunOrder returns a map of country → Grand Final running order draw number.
// Returns an error if the draw hasn't been published on Wikipedia yet.
func FetchFinalRunOrder(year int) (map[string]int, error) {
	base := "https://en.wikipedia.org/wiki/Eurovision_Song_Contest_" +
		strconv.Itoa(year) + "_%E2%80%93_Grand_final"

	// Try dedicated Grand Final article pages first.
	for _, url := range []string{base, strings.Replace(base, "Grand_final", "Grand_Final", 1)} {
		doc, err := fetchDocURL(url)
		if err != nil {
			continue
		}
		if order := parseRunOrderFromRows(doc.Find("table.wikitable tr")); len(order) > 0 {
			return order, nil
		}
	}

	// Fall back to the main contest article, scoped to the Grand Final section
	// so we don't accidentally read semi-final draw numbers.
	doc, err := fetchDoc(year)
	if err != nil {
		return nil, fmt.Errorf("could not fetch Eurovision page: %w", err)
	}
	if order := parseRunOrderFromFinalSection(doc); len(order) > 0 {
		return order, nil
	}

	return nil, fmt.Errorf("Grand Final running order not yet available on Wikipedia")
}

// parseRunOrderFromFinalSection finds the "Final" heading in the main article
// and reads running order numbers only from the tables within that section.
func parseRunOrderFromFinalSection(doc *goquery.Document) map[string]int {
	var rows *goquery.Selection
	doc.Find("h2, h3, .mw-heading").EachWithBreak(func(_ int, h *goquery.Selection) bool {
		text := strings.ToLower(cleanText(h.Text()))
		// Match "Final" or "Grand Final" but not "Semi-final".
		if !strings.Contains(text, "final") || strings.Contains(text, "semi") {
			return true
		}
		anchor := h
		if h.Parent().HasClass("mw-heading") {
			anchor = h.Parent()
		}
		anchor.NextUntil("h2, h3, .mw-heading").Each(func(_ int, s *goquery.Selection) {
			// The table is often the direct sibling element, not a descendant,
			// so we must check whether s itself is the table.
			var found *goquery.Selection
			if goquery.NodeName(s) == "table" {
				found = s.Find("tr")
			} else {
				found = s.Find("table.wikitable tr")
			}
			if rows == nil {
				rows = found
			} else {
				rows = rows.AddSelection(found)
			}
		})
		return false
	})
	if rows == nil {
		return nil
	}
	return parseRunOrderFromRows(rows)
}

// parseRunOrderFromRows extracts country → draw number from a set of table rows.
// A row contributes when its th[scope=row] is a positive integer and the first td
// holds a non-empty country name.
func parseRunOrderFromRows(rows *goquery.Selection) map[string]int {
	order := make(map[string]int)
	rows.Each(func(_ int, row *goquery.Selection) {
		if row.Find("th[scope=col]").Length() > 0 {
			return
		}
		th := cleanText(row.Find("th[scope=row]").First().Text())
		n, err := strconv.Atoi(th)
		if err != nil || n < 1 {
			return
		}
		country := cleanText(row.Find("td").First().Text())
		if country != "" && order[country] == 0 {
			order[country] = n
		}
	})
	return order
}

// FetchFinalRankings fetches the Grand Final placement results.
// Returns an error if results aren't available yet.
func FetchFinalRankings(year int) ([]models.FinalEntry, error) {
	base := "https://en.wikipedia.org/wiki/Eurovision_Song_Contest_" +
		strconv.Itoa(year) + "_%E2%80%93_Grand_final"

	// Try lowercase then uppercase "final" as Wikipedia naming varies by year.
	var doc *goquery.Document
	var fetchErr error
	for _, url := range []string{base, strings.Replace(base, "Grand_final", "Grand_Final", 1)} {
		doc, fetchErr = fetchDocURL(url)
		if fetchErr == nil {
			break
		}
	}
	if fetchErr != nil {
		// Fall back to the main contest article.
		doc, fetchErr = fetchDoc(year)
		if fetchErr != nil {
			return nil, fmt.Errorf("results not yet available — %w", fetchErr)
		}
	}

	rankings := parseFinalRankings(doc)
	if len(rankings) == 0 {
		return nil, fmt.Errorf("results not yet available on Wikipedia")
	}
	return rankings, nil
}

// parseQualifiers scans all wikitables in doc for qualifier rows.
func parseQualifiers(doc *goquery.Document) []string {
	return collectQualifiers(doc.Find("table.wikitable tr"))
}

// parseQualifiersFromSection scans only the table(s) inside the semi-final
// section matching semiNum in the main contest article.
func parseQualifiersFromSection(doc *goquery.Document, semiNum int) []string {
	target := "semi-final 1"
	if semiNum == 2 {
		target = "semi-final 2"
	}

	var rows *goquery.Selection
	doc.Find("h2, h3, .mw-heading").EachWithBreak(func(_ int, h *goquery.Selection) bool {
		if !strings.Contains(strings.ToLower(cleanText(h.Text())), target) {
			return true
		}
		anchor := h
		if h.Parent().HasClass("mw-heading") {
			anchor = h.Parent()
		}
		anchor.NextUntil("h2, h3, .mw-heading").Each(func(_ int, s *goquery.Selection) {
			found := s.Find("table.wikitable tr")
			if rows == nil {
				rows = found
			} else {
				rows = rows.AddSelection(found)
			}
		})
		return false
	})

	if rows == nil {
		return nil
	}
	return collectQualifiers(rows)
}

// collectQualifiers extracts qualifier country names from a set of table rows.
// Handles two table layouts:
//   - Country in th[scope=row] (older standalone semi-final articles)
//   - Running-order number in th[scope=row], country in first td (2026 main article)
func collectQualifiers(rows *goquery.Selection) []string {
	var qualifiers []string
	seen := make(map[string]bool)

	rows.Each(func(_ int, row *goquery.Selection) {
		if row.Find("th[scope=col]").Length() > 0 {
			return
		}
		country := extractCountryFromRow(row)
		if country == "" || seen[country] {
			return
		}

		// Text-based markers: "Q", "Qualified", "✓"
		row.Find("td").Each(func(_ int, td *goquery.Selection) {
			if seen[country] {
				return
			}
			text := cleanText(td.Text())
			if strings.EqualFold(text, "q") || strings.EqualFold(text, "qualified") || text == "✓" {
				qualifiers = append(qualifiers, country)
				seen[country] = true
			}
		})
		if seen[country] {
			return
		}

		// Visual marker: qualifying rows have background:navajowhite or a table-yes class.
		if rowIsQualified(row) {
			qualifiers = append(qualifiers, country)
			seen[country] = true
		}
	})

	return qualifiers
}

// extractCountryFromRow returns the country name from a table row.
// In 2026-style tables the th[scope=row] holds the running-order number,
// so we fall back to the first td when that's the case.
func extractCountryFromRow(row *goquery.Selection) string {
	th := cleanText(row.Find("th[scope=row]").First().Text())
	if _, err := strconv.Atoi(th); err == nil {
		return cleanText(row.Find("td").First().Text())
	}
	return th
}

// rowIsQualified returns true when a table row is visually marked as qualified.
// Wikipedia uses background:navajowhite (Eurovision 2026) or table-yes CSS classes.
func rowIsQualified(row *goquery.Selection) bool {
	class, _ := row.Attr("class")
	if strings.Contains(class, "table-yes") {
		return true
	}
	style, _ := row.Attr("style")
	style = strings.ToLower(strings.ReplaceAll(style, " ", ""))
	for _, indicator := range []string{
		"navajowhite",
		"#9eff9e", "#aaffaa", "#aafcaa", "#9ef09e", "#90ee90", "#dfd", "#cfc", "#afa",
	} {
		if strings.Contains(style, indicator) {
			return true
		}
	}
	return false
}

// parseFinalRankings looks for a results wikitable with a "Place"/"#"/"Rank" column
// and extracts country → place pairs.
func parseFinalRankings(doc *goquery.Document) []models.FinalEntry {
	var entries []models.FinalEntry

	doc.Find("table.wikitable").EachWithBreak(func(_ int, table *goquery.Selection) bool {
		headers := table.Find("th").Map(func(_ int, th *goquery.Selection) string {
			return strings.ToLower(cleanText(th.Text()))
		})
		if !containsAll(headers, "country") {
			return true
		}
		hasPlace := false
		for _, h := range headers {
			if h == "place" || h == "#" || h == "rank" || strings.HasPrefix(h, "place") {
				hasPlace = true
				break
			}
		}
		if !hasPlace {
			return true
		}

		var tableEntries []models.FinalEntry
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			if row.Find("th[scope=col]").Length() > 0 {
				return
			}
			country := cleanText(row.Find("th[scope=row]").First().Text())
			if country == "" {
				return
			}
			// Find the first td with a small integer — assumed to be the placement.
			row.Find("td").EachWithBreak(func(_ int, td *goquery.Selection) bool {
				n, err := strconv.Atoi(cleanText(td.Text()))
				if err == nil && n >= 1 && n <= 30 {
					tableEntries = append(tableEntries, models.FinalEntry{Place: n, Country: country})
					return false
				}
				return true
			})
		})

		if len(tableEntries) > 10 {
			sort.Slice(tableEntries, func(i, j int) bool {
				return tableEntries[i].Place < tableEntries[j].Place
			})
			entries = tableEntries
			return false
		}
		return true
	})

	return entries
}
