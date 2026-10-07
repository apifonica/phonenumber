package phonenumber

import (
	"regexp"
	"strings"
	"sync"
)

var (
	digitsOnlyRegexp         = regexp.MustCompile(`\D`)
	leadZeroRegexp           = regexp.MustCompile(`^0+`)
	rusLocalePrefixRegexp    = regexp.MustCompile(`^8+`)
	rusLocaleMobPrefixRegexp = regexp.MustCompile(`^89`)
)

// Parse mobile number by country
func Parse(number string, country string) string {
	parsed, iso3166 := parseInternal(number, country)
	if validateMobileISO3166(parsed, iso3166) {
		return parsed
	}
	return ""
}

// ParseWithLandLine is Parse mobile and landline number by country
func ParseWithLandLine(number string, country string) string {
	parsed, iso3166 := parseInternal(number, country)
	if validateLandlineISO3166(parsed, iso3166) {
		return parsed
	}
	return ""
}

// ParseWithFlags parses the number and returns two flags, indicating
// whether the number is valid and whether it is mobile.
func ParseWithFlags(number string, country string) (parsed string, valid bool, mobile bool) {
	var iso3166 ISO3166
	parsed, iso3166 = parseInternal(number, country)
	valid, mobile = validatePhoneISO3166(parsed, iso3166)
	if !valid {
		parsed = ""
	}
	return
}

// GetISO3166ByNumber ...
func GetISO3166ByNumber(number string, withLandLine bool) ISO3166 {
	iso3166 := ISO3166{}
	for _, i := range GetISO3166() {
		r := getRegexpByCountryCode(i.CountryCode)
		for _, l := range i.PhoneNumberLengths {
			if r.MatchString(number) && len(number) == len(i.CountryCode)+l {
				// Check match with mobile codes
				if i.mobilePrefixes.hasPrefixOf(number[len(i.CountryCode):], 0) {
					return i
				}

				// Match by country code only for landline numbers only
				if withLandLine {
					iso3166 = i
					break
				}
			}
		}
	}
	return iso3166
}

// GetISO3166ByMobileNumber ...
func GetISO3166ByMobileNumber(number string) []ISO3166 {
	result := []ISO3166{}
	for _, i := range GetISO3166() {
		for _, l := range i.PhoneNumberLengths {
			if len(number) == l && i.mobilePrefixes.hasPrefixOf(number, 1) {
				result = append(result, i)
			}
		}
	}
	return result
}

func parseInternal(number string, country string) (string, ISO3166) {
	number = strings.ReplaceAll(number, " ", "")
	country = strings.ReplaceAll(country, " ", "")

	if strings.HasPrefix(number, "+") {
		if country == "" {
			return "", ISO3166{}
		}
	}

	// remove any non-digit character, included the +
	number = digitsOnlyRegexp.ReplaceAllString(number, "")

	iso3166 := getISO3166ByCountry(country)

	// Track if we stripped a leading zero (indicates local format input)
	hadLeadingZero := strings.HasPrefix(number, "0")

	// Strip leading zeros first (except for specific countries)
	if indexOfString(iso3166.Alpha3, []string{"GAB", "CIV", "COG"}) == -1 {
		number = leadZeroRegexp.ReplaceAllString(number, "")
	}

	// Handle Russia-specific local format
	if iso3166.Alpha3 == "RUS" && len(number) == 11 && rusLocaleMobPrefixRegexp.MatchString(number) {
		number = rusLocalePrefixRegexp.ReplaceAllString(number, "")
	}

	// Check if number is already a valid international number (has country code + valid local length)
	// This handles edge cases where MobileBeginWith prefixes start with the same digits as CountryCode
	if isValidInternationalLength(number, iso3166) {
		return number, iso3166
	}

	startsWithCountryCode := strings.HasPrefix(number, iso3166.CountryCode)

	// Prepend country code if:
	// - number does NOT start with country code, OR
	// - original input had leading zero (local format like "07723456789")
	// AND length matches a valid local length
	if (!startsWithCountryCode || hadLeadingZero) && indexOfInt(len(number), iso3166.PhoneNumberLengths) != -1 {
		number = iso3166.CountryCode + number
		return number, iso3166
	}

	// Handle case where number has country code followed by leading zero (e.g., "490301234567" → "49301234567")
	if startsWithCountryCode {
		withoutCountryCode := strings.Replace(number, iso3166.CountryCode, "", 1)
		// Only strip leading zero if it results in a valid local length
		if strings.HasPrefix(withoutCountryCode, "0") {
			withoutZero := strings.Replace(withoutCountryCode, "0", "", 1)
			if indexOfInt(len(withoutZero), iso3166.PhoneNumberLengths) != -1 {
				number = iso3166.CountryCode + withoutZero
			}
		}
	}

	return number, iso3166
}

// isValidInternationalLength checks if a number starts with country code and has valid total length
func isValidInternationalLength(number string, iso3166 ISO3166) bool {
	if !strings.HasPrefix(number, iso3166.CountryCode) {
		return false
	}
	localLen := len(number) - len(iso3166.CountryCode)
	return indexOfInt(localLen, iso3166.PhoneNumberLengths) != -1
}

func getISO3166ByCountry(country string) ISO3166 {
	iso3166 := ISO3166{}
	uppperCaseCountry := strings.ToUpper(country)
	switch len(country) {
	case 0:
		iso3166 = GetISO3166()[0]
	case 2:
		for _, i := range GetISO3166() {
			if i.Alpha2 == uppperCaseCountry {
				iso3166 = i
				break
			}
		}
	case 3:
		for _, i := range GetISO3166() {
			if i.Alpha3 == uppperCaseCountry {
				iso3166 = i
				break
			}
		}
	default:
		for _, i := range GetISO3166() {
			if strings.ToUpper(i.CountryName) == uppperCaseCountry {
				iso3166 = i
				break
			}
		}
	}
	return iso3166
}

func validateMobileISO3166(number string, iso3166 ISO3166) bool {
	if len(iso3166.PhoneNumberLengths) == 0 {
		return false
	}

	r := getRegexpByCountryCode(iso3166.CountryCode)
	number = r.ReplaceAllString(number, "")
	for _, l := range iso3166.PhoneNumberLengths {
		if l == len(number) && iso3166.mobilePrefixes.hasPrefixOf(number, 0) {
			return true
		}
	}
	return false
}

func validateLandlineISO3166(number string, iso3166 ISO3166) bool {
	if len(iso3166.PhoneNumberLengths) == 0 {
		return false
	}

	r := getRegexpByCountryCode(iso3166.CountryCode)
	for _, l := range iso3166.PhoneNumberLengths {
		if r.MatchString(number) && len(number) == len(iso3166.CountryCode)+l {
			return true
		}
	}
	return false
}

func validatePhoneISO3166(number string, iso3166 ISO3166) (valid bool, mobile bool) {
	if !validateLandlineISO3166(number, iso3166) {
		valid = false
		mobile = false
		return
	}

	// Landline check passed, but maybe number is mobile
	valid = true

	if validateMobileISO3166(number, iso3166) {
		// Mobile check passed
		mobile = true
	}

	return
}

func indexOfString(word string, data []string) int {
	for k, v := range data {
		if word == v {
			return k
		}
	}
	return -1
}

func indexOfInt(word int, data []int) int {
	for k, v := range data {
		if word == v {
			return k
		}
	}
	return -1
}

// prefixSet indexes MobileBeginWith so a lookup costs one map probe per
// number digit, not one comparison per prefix (MX alone has tens of thousands).
type prefixSet struct {
	prefixes map[string]struct{}
	maxLen   int
}

func newPrefixSet(prefixes []string) prefixSet {
	s := prefixSet{prefixes: make(map[string]struct{}, len(prefixes))}
	for _, p := range prefixes {
		s.prefixes[p] = struct{}{}
		s.maxLen = max(s.maxLen, len(p))
	}
	return s
}

// hasPrefixOf reports whether number starts with one of the prefixes that is
// at least minLen digits long; with minLen 0 an empty prefix matches anything.
func (s prefixSet) hasPrefixOf(number string, minLen int) bool {
	for l := minLen; l <= min(len(number), s.maxLen); l++ {
		if _, ok := s.prefixes[number[:l]]; ok {
			return true
		}
	}
	return false
}

var rMap = map[string]*regexp.Regexp{}
var rLock = sync.RWMutex{}

func getRegexpByCountryCode(countryCode string) *regexp.Regexp {
	rLock.Lock()
	defer rLock.Unlock()
	regex, exists := rMap[countryCode]
	if exists {
		return regex
	} else {
		rMap[countryCode] = regexp.MustCompile(`^` + countryCode)
	}
	return rMap[countryCode]
}
