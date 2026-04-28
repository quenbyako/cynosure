package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

//nolint:lll // we can't shorten the link.
const (
	ipv4SpecialPurposeRegistry = "https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry-1.csv"
	ipv6SpecialPurposeRegistry = "https://www.iana.org/assignments/iana-ipv6-special-registry/iana-ipv6-special-registry-1.csv"
	ipv6GlobalUnicast          = "2000::/3"
)

var additionalV4Entries = []entry{
	{Name: "Multicast", Prefix: "224.0.0.0/4", RFC: "RFC 1112, Section 4"},
}

//nolint:funlen // generator entry point
func main() {
	output := flag.String("output.gen", "", "file to write the generated code into")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := loadTemplates(); err != nil {
		errExit(err)
	}

	ipv4, err := fetch(ctx, ipv4SpecialPurposeRegistry)
	if err != nil {
		errExit(err)
	}

	ipv6, err := fetch(ctx, ipv6SpecialPurposeRegistry)
	if err != nil {
		errExit(err)
	}

	const outputPerms = 0o644

	outputFile, err := os.OpenFile(*output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, outputPerms)
	if err != nil {
		errExit(err, outputFile)
	}

	//nolint:errcheck // we don't care about closing errors for write-only files in generator
	defer outputFile.Close()

	tmpl, ok := templates["ssrf.tmpl"]
	if !ok {
		return
	}

	data := struct {
		V6GlobalUnicast string
		V4              []entry
		V6              []entry
	}{
		V4:              append(ipv4, additionalV4Entries...),
		V6:              ipv6,
		V6GlobalUnicast: ipv6GlobalUnicast,
	}

	if err := tmpl.Execute(outputFile, data); err != nil {
		errExit(err, outputFile)
	}

	//nolint:gosec // false positive: codegen must be formatted.
	if res, err := exec.CommandContext(ctx, "go", "fmt", *output).CombinedOutput(); err != nil {
		//nolint:forbidigo // logging generator failure
		fmt.Println(string(res))
	}
}

// cleanRFC tries to clean up the RFC field from the IANA Special Purpose
// registry CSV and turn it into something consistent
func cleanRFC(str string) string {
	str = strings.ReplaceAll(str, "\n", ",")
	str = strings.ReplaceAll(str, "][", ", ")
	str = strings.ReplaceAll(str, "[", "")
	str = strings.ReplaceAll(str, "]", "")
	str = strings.ReplaceAll(str, "RFC", "RFC ")
	str = strings.Join(strings.Fields(str), " ")

	return str
}

// cleanName does some small transformations on the Name of a prefix
func cleanName(s string) string {
	return strings.ReplaceAll(s, "Translat.", "Translation")
}

// errExit prints the error, attempts to close any passed in files and then
// exits with the provided code
func errExit(err error, files ...*os.File) {
	//nolint:forbidigo // exits the program with an error code.
	fmt.Fprintln(os.Stderr, err)

	for _, f := range files {
		//nolint:errcheck,gosec // we don't care about the error here.
		f.Close()
	}

	//nolint:forbidigo // exits the program with an error code.
	os.Exit(1)
}

// handleNetwork is used to deal with the fact that a Prefix from the IANA
// Special Purpose registry can contain more than one prefix
func handleNetwork(s string) []string {
	list := strings.Split(s, ",")
	res := []string{}

	for _, item := range list {
		item = strings.TrimSpace(item)

		i := strings.Index(item, " ")
		if i == -1 {
			res = append(res, item)
		} else {
			res = append(res, item[:i])
		}
	}

	return res
}

// entry represent a single prefix from a IANA Special Purpose registry
type entry struct {
	Prefix string
	Name   string
	RFC    string
}

// fetch retrieves a particular IANA Special Purpose registry and parses the
// returned CSV into [Entry]s.
//
// This function deduplicates prefixes and calls a number of cleaner functions
// on the data.
func fetch(ctx context.Context, url string) ([]entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("could not create request for %s: %w", url, err)
	}

	req.Header.Set("User-Agent", "ssrfgen (+https://code.dny.dev/ssrf")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request for %s: %w", url, err)
	}

	//nolint:errcheck,gosec // we don't care about the error here.
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	r := csv.NewReader(resp.Body)

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse record in %s: %w", url, err)
	}

	return processRecords(records), nil
}

func processRecords(records [][]string) []entry {
	entries := []entry{}

	for _, rec := range records[1:] {
		for _, prefixRaw := range handleNetwork(rec[0]) {
			prefix := netip.MustParsePrefix(prefixRaw)
			if prefix.Addr().Is4() {
				entries = processIPv4Record(entries, prefixRaw, rec)
			} else if prefix.Addr().Is6() {
				entries = processIPv6Record(entries, prefixRaw, rec)
			}
		}
	}

	return entries
}

func processIPv4Record(entries []entry, prefixRaw string, rec []string) []entry {
	if containsPrefix(entries, prefixRaw) {
		fmt.Printf("Skipping prefix: %s as it's already matched\n", prefixRaw)
		return entries
	}

	return append(entries, entry{
		Prefix: prefixRaw,
		Name:   cleanName(rec[1]),
		RFC:    cleanRFC(rec[2]),
	})
}

func processIPv6Record(entries []entry, prefixRaw string, rec []string) []entry {
	if !containsPrefix([]entry{{Prefix: ipv6GlobalUnicast, Name: "", RFC: ""}}, prefixRaw) {
		fmt.Printf("Skipping prefix: %s as it's not within Global Unicast range\n", prefixRaw)
		return entries
	}

	if containsPrefix(entries, prefixRaw) {
		fmt.Printf("Skipping prefix: %s as it's already matched\n", prefixRaw)
		return entries
	}

	return append(entries, entry{
		Prefix: prefixRaw,
		Name:   cleanName(rec[1]),
		RFC:    cleanRFC(rec[2]),
	})
}

// containsPrefix checks if a prefix we're encountering is already matched by
// a previous entry.
//
// The IANA registries are sorted by prefix, so a larger prefix will show up
// before a smaller one. This means we can simply iterate over the list.
func containsPrefix(entries []entry, prefix string) bool {
	prefix2 := netip.MustParsePrefix(prefix)

	found := false

	for _, e := range entries {
		prefix1 := netip.MustParsePrefix(e.Prefix)
		if prefix2.Bits() < prefix1.Bits() {
			continue
		}

		pp, err := prefix2.Addr().Prefix(prefix1.Bits())
		if err != nil {
			return false // This should never happen unless we're mix-matching v4 and v6
		}

		found = pp.Addr() == prefix1.Addr()
		if found {
			break
		}
	}

	return found
}
