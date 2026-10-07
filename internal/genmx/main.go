// Command genmx regenerates the Mexican mobile prefixes from the IFT
// Plan Nacional de Numeración (PNN). Mexico has no mobile-specific NDCs:
// mobile (CPP/MPP) and fixed (FIJO) lines share area codes and differ only
// by the blocks they are assigned, so every CPP/MPP block becomes a prefix.
package main

import (
	"archive/zip"
	"bytes"
	"cmp"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	pnnURL    = "https://sns.ift.org.mx/sns-frontend/planes-numeracion/descarga-publica.xhtml"
	nsnLength = 10
)

var viewStateRegexp = regexp.MustCompile(`name="javax\.faces\.ViewState"[^>]*value="([^"]+)"`)

type numberRange struct {
	first, last int64
}

func main() {
	src := flag.String("src", "", "PNN .zip or .csv to read instead of downloading it from IFT")
	out := flag.String("out", "iso3166_mx.go", "generated Go file")
	flag.Parse()

	name, data, err := load(*src)
	if err != nil {
		log.Fatal(err)
	}

	ranges, err := parseMobileRanges(data)
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	prefixes := coverWithPrefixes(mergeRanges(ranges))

	code, err := render(name, prefixes)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, code, 0o644); err != nil {
		log.Fatal(err)
	}

	log.Printf("%s: %d mobile prefixes written to %s", name, len(prefixes), *out)
}

// load returns the PNN CSV file name and contents.
func load(src string) (string, []byte, error) {
	if strings.HasSuffix(src, ".csv") {
		data, err := os.ReadFile(src)
		return filepath.Base(src), data, err
	}

	var archive []byte
	var err error
	if src == "" {
		archive, err = download()
	} else {
		archive, err = os.ReadFile(src)
	}
	if err != nil {
		return "", nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", nil, err
	}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		return f.Name, data, err
	}
	return "", nil, errors.New("no .csv file in the PNN archive")
}

// download submits the JSF form behind the "Descargar" button of the public
// PNN page; the archive has no stable URL of its own.
func download() ([]byte, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Jar: jar, Timeout: 2 * time.Minute}

	page, err := fetch(client.Get(pnnURL))
	if err != nil {
		return nil, err
	}
	m := viewStateRegexp.FindSubmatch(page)
	if m == nil {
		return nil, fmt.Errorf("javax.faces.ViewState not found on %s", pnnURL)
	}

	return fetch(client.PostForm(pnnURL, url.Values{
		"FORM_planes":                  {"FORM_planes"},
		"FORM_planes:BTN_planPublico1": {""},
		"javax.faces.ViewState":        {html.UnescapeString(string(m[1]))},
	}))
}

func fetch(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: %s", resp.Request.Method, resp.Request.URL, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// parseMobileRanges reads the PNN CSV (layout since July 2025: ZONA,
// NUMERACION_INICIAL, NUMERACION_FINAL, OCUPACION, MODALIDAD, RAZON_SOCIAL,
// FECHA_ASIGNACION) and returns the CPP and MPP ranges.
func parseMobileRanges(data []byte) ([]numberRange, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	columns := map[string]int{}
	for i, h := range header {
		columns[strings.TrimSpace(h)] = i
	}
	firstCol, ok1 := columns["NUMERACION_INICIAL"]
	lastCol, ok2 := columns["NUMERACION_FINAL"]
	modalityCol, ok3 := columns["MODALIDAD"]
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("unexpected PNN layout, header: %q", header)
	}

	var ranges []numberRange
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		line, _ := r.FieldPos(0)

		if len(record) <= max(firstCol, lastCol, modalityCol) {
			return nil, fmt.Errorf("line %d: %d fields", line, len(record))
		}
		first, err := parseNSN(record[firstCol])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		last, err := parseNSN(record[lastCol])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if first > last {
			return nil, fmt.Errorf("line %d: range %d-%d is reversed", line, first, last)
		}

		switch modality := strings.TrimSpace(record[modalityCol]); modality {
		case "CPP", "MPP":
			ranges = append(ranges, numberRange{first: first, last: last})
		case "FIJO":
		default:
			return nil, fmt.Errorf("line %d: unknown MODALIDAD %q", line, modality)
		}
	}
	if len(ranges) == 0 {
		return nil, errors.New("no CPP/MPP ranges")
	}
	return ranges, nil
}

func parseNSN(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if len(s) != nsnLength {
		return 0, fmt.Errorf("number %q is not %d digits long", s, nsnLength)
	}
	return strconv.ParseInt(s, 10, 64)
}

func mergeRanges(ranges []numberRange) []numberRange {
	slices.SortFunc(ranges, func(a, b numberRange) int {
		return cmp.Compare(a.first, b.first)
	})

	merged := ranges[:1]
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.first <= last.last+1 {
			last.last = max(last.last, r.last)
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// coverWithPrefixes splits each range into the fewest NSN prefixes that
// cover exactly that range, e.g. 5512340000-5512359999 -> "551234", "551235".
func coverWithPrefixes(ranges []numberRange) []string {
	var prefixes []string
	for _, r := range ranges {
		for n := r.first; n <= r.last; {
			size, wildcards := int64(1), 0
			for n%(size*10) == 0 && n+size*10-1 <= r.last {
				size *= 10
				wildcards++
			}
			prefixes = append(prefixes, strconv.FormatInt(n, 10)[:nsnLength-wildcards])
			n += size
		}
	}
	return prefixes
}

// render writes one line per 4-digit group, so a refresh diffs by area
// rather than reflowing the whole list.
func render(source string, prefixes []string) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by go run ./internal/genmx; DO NOT EDIT.\n")
	fmt.Fprintf(&b, "// Source: IFT Plan Nacional de Numeración, %s.\n\n", source)
	b.WriteString("package phonenumber\n\nvar mxMobileBeginWith = []string{")

	group := func(p string) string { return p[:min(len(p), 4)] }
	for i, p := range prefixes {
		if i == 0 || group(p) != group(prefixes[i-1]) {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%q,", p)
	}
	b.WriteString("\n}\n")

	return format.Source(b.Bytes())
}
