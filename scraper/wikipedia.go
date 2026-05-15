package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"eurovote/models"

	"github.com/PuerkitoBio/goquery"
)

// big5 are the countries that automatically qualify every year.
var big5 = map[string]bool{
	"France":         true,
	"Germany":        true,
	"Italy":          true,
	"Spain":          true,
	"United Kingdom": true,
}

var footnoteRe = regexp.MustCompile(`\[\d+\]`)
var dateRe = regexp.MustCompile(`\b(\d{1,2}) (January|February|March|April|May|June|July|August|September|October|November|December) (\d{4})\b`)

func cleanText(s string) string {
	s = footnoteRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// FetchContest scrapes the Eurovision Wikipedia article for the given year and
// returns all acts and the specific contest dates in a single HTTP request.
func FetchContest(year int) ([]models.Act, models.ContestDates, error) {
	doc, err := fetchDoc(year)
	if err != nil {
		return nil, models.ContestDates{}, err
	}

	sfMap := buildSemiFinaleMap(doc)
	hostCountry := detectHost(doc)

	acts := parseParticipantsTable(doc, sfMap, hostCountry)
	if len(acts) == 0 {
		return nil, models.ContestDates{}, fmt.Errorf("no acts found — Wikipedia page structure may have changed")
	}

	dates, err := parseDates(doc)
	if err != nil {
		return nil, models.ContestDates{}, err
	}

	return acts, dates, nil
}

func fetchDocURL(rawURL string) (*goquery.Document, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Eurovote/1.0 (educational app)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Wikipedia: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Wikipedia returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	return doc, nil
}

func fetchDoc(year int) (*goquery.Document, error) {
	url := fmt.Sprintf("https://en.wikipedia.org/wiki/Eurovision_Song_Contest_%d", year)
	return fetchDocURL(url)
}

// dateInText extracts the first "D Month YYYY" date found anywhere in s.
// Uses a regex so it works even when the date is embedded in surrounding CSS text.
func dateInText(s string) (time.Time, bool) {
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	return tryParseDate(m[1] + " " + m[2] + " " + m[3])
}

// parseDates finds the SF1, SF2, and Final dates from the Wikipedia infobox.
// Handles two layouts:
//   - Old: one "Dates" row with all events listed in the TD.
//   - New (2026+): separate rows per event with TH = "Semi-final 1" etc.
func parseDates(doc *goquery.Document) (models.ContestDates, error) {
	var cd models.ContestDates

	doc.Find(".infobox tr").Each(func(_ int, row *goquery.Selection) {
		th := strings.ToLower(cleanText(row.Find("th").Text()))
		tdText := row.Find("td").Text()

		// New layout: each event has its own row.
		var label string
		switch {
		case strings.Contains(th, "semi-final 1") || strings.Contains(th, "first semi-final"):
			label = "sf1"
		case strings.Contains(th, "semi-final 2") || strings.Contains(th, "second semi-final"):
			label = "sf2"
		case strings.Contains(th, "grand final") || (strings.Contains(th, "final") && !strings.Contains(th, "semi")):
			label = "final"
		}

		if label != "" {
			if t, ok := dateInText(tdText); ok {
				switch label {
				case "sf1":
					cd.SF1 = t
				case "sf2":
					cd.SF2 = t
				case "final":
					cd.Final = t
				}
			}
			return
		}

		// Old layout: one "Dates" row with all events in the TD.
		if !strings.Contains(th, "date") {
			return
		}

		lines := strings.Split(strings.TrimSpace(tdText), "\n")
		var lastLabel string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			low := strings.ToLower(line)
			switch {
			case strings.Contains(low, "semi-final 1") || strings.Contains(low, "first semi-final"):
				lastLabel = "sf1"
			case strings.Contains(low, "semi-final 2") || strings.Contains(low, "second semi-final"):
				lastLabel = "sf2"
			case strings.Contains(low, "grand final") || strings.Contains(low, "final"):
				lastLabel = "final"
			default:
				if t, ok := tryParseDate(line); ok {
					switch lastLabel {
					case "sf1":
						cd.SF1 = t
					case "sf2":
						cd.SF2 = t
					case "final":
						cd.Final = t
					}
				}
			}
		}
	})

	if cd.SF1.IsZero() || cd.SF2.IsZero() || cd.Final.IsZero() {
		return cd, fmt.Errorf("could not parse all Eurovision dates from Wikipedia — page structure may have changed")
	}
	return cd, nil
}

func tryParseDate(s string) (time.Time, bool) {
	for _, format := range []string{"2 January 2006", "2 Jan 2006"} {
		if t, err := time.ParseInLocation(format, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// buildSemiFinaleMap scans section headers for "Semi-final 1" and "Semi-final 2"
// and collects country names from the tables that follow them.
func buildSemiFinaleMap(doc *goquery.Document) map[string]int {
	sfMap := make(map[string]int)

	doc.Find("h2, h3").Each(func(_ int, h *goquery.Selection) {
		text := strings.ToLower(cleanText(h.Text()))

		var group int
		if strings.Contains(text, "semi-final 1") || strings.Contains(text, "first semi-final") {
			group = 1
		} else if strings.Contains(text, "semi-final 2") || strings.Contains(text, "second semi-final") {
			group = 2
		} else {
			return
		}

		// Modern Wikipedia wraps headings in <div class="mw-heading">. Use that
		// wrapper as the sibling-traversal anchor so tables at the same DOM level
		// are reachable. Fall back to the heading element itself for older pages.
		anchor := h.Parent()
		if !anchor.HasClass("mw-heading") {
			anchor = h
		}

		anchor.NextUntil("h2, h3, .mw-heading").Each(func(_ int, s *goquery.Selection) {
			s.Find("tr").Each(func(_ int, row *goquery.Selection) {
				th := cleanText(row.Find("th[scope=row]").First().Text())
				country := th
				// 2026-style running-order tables put the R/O number in th[scope=row];
				// the country name is in the first td.
				if isAllDigits(th) {
					country = cleanText(row.Find("td").First().Text())
				}
				if country != "" {
					sfMap[country] = group
				}
			})
		})
	})

	return sfMap
}

// detectHost tries to find the host country from the article intro paragraph.
func detectHost(doc *goquery.Document) string {
	hostedByRe := regexp.MustCompile(`(?i)hosted (?:by|in) ([A-Z][a-z]+(?:\s[A-Z][a-z]+)*)`)
	var host string
	doc.Find("#mw-content-text > .mw-parser-output > p").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if i > 5 {
			return false
		}
		m := hostedByRe.FindStringSubmatch(s.Text())
		if len(m) > 1 {
			host = m[1]
			return false
		}
		return true
	})
	return host
}

// parseParticipantsTable extracts acts from the main wikitable and assigns
// semi-final groups using sfMap and host/Big 5 logic.
func parseParticipantsTable(doc *goquery.Document, sfMap map[string]int, hostCountry string) []models.Act {
	var acts []models.Act

	doc.Find("table.wikitable").EachWithBreak(func(_ int, table *goquery.Selection) bool {
		// Identify the participants table by checking for expected column headers.
		headers := table.Find("th").Map(func(_ int, th *goquery.Selection) string {
			return strings.ToLower(cleanText(th.Text()))
		})
		if !containsAll(headers, "country", "artist", "song") {
			return true // continue to next table
		}

		var tableActs []models.Act
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			// Skip header rows.
			if row.Find("th[scope=col]").Length() > 0 {
				return
			}

			country := cleanText(row.Find("th[scope=row]").First().Text())
			if country == "" {
				return
			}

			tds := row.Find("td")
			artist := cleanText(tds.Eq(1).Text())
			song := cleanText(tds.Eq(2).Text())

			// Strip surrounding quotes from song titles.
			song = strings.Trim(song, `"`)

			if artist == "" || song == "" {
				return
			}

			group := sfMap[country]
			_ = big5[country] || strings.EqualFold(country, hostCountry) // auto-qualifiers keep group = 0

			tableActs = append(tableActs, models.Act{
				Country:   country,
				Artist:    artist,
				Song:      song,
				SemiGroup: group,
			})
		})

		if len(tableActs) > 0 {
			acts = tableActs
			return false // stop — don't parse additional tables
		}
		return true
	})

	return acts
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func containsAll(haystack []string, needles ...string) bool {
	for _, n := range needles {
		found := false
		for _, h := range haystack {
			if strings.Contains(h, n) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
