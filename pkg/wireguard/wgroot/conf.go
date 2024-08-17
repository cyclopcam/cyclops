package wgroot

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Parse and write wireguard config file

var regexpSection *regexp.Regexp
var regexpValue *regexp.Regexp

type configFile struct {
	sections []*configSection
}

type configSection struct {
	title string // eg Interface or Peer
	lines []configLine
}

type configLine struct {
	key   string
	value string
}

// Load a config file
func loadConfigFile(path string) (*configFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cf := &configFile{}
	var section *configSection
	for _, line := range strings.Split(string(raw), "\n") {
		// We do not preserve whitespace or comments
		line = strings.TrimSpace(line)
		if hasContent(line) {
			matches := regexpSection.FindStringSubmatch(line)
			if len(matches) == 2 {
				section = &configSection{
					title: matches[1],
				}
				cf.sections = append(cf.sections, section)
			} else if matches = regexpValue.FindStringSubmatch(line); len(matches) == 3 {
				// ignore any lines that appear before a [section] header
				if section != nil {
					section.lines = append(section.lines, configLine{
						key:   strings.TrimSpace(matches[1]),
						value: strings.TrimSpace(matches[2]),
					})
				}
			}
		}
	}
	return cf, nil
}

// Write to file
func (f *configFile) writeFile(filename string) error {
	sb := bytes.Buffer{}
	for isection, section := range f.sections {
		if isection != 0 {
			fmt.Fprintf(&sb, "\n")
		}
		fmt.Fprintf(&sb, "[%v]\n", section.title)
		for _, line := range section.lines {
			fmt.Fprintf(&sb, "%v = %v\n", line.key, line.value)
		}
	}
	return os.WriteFile(filename, sb.Bytes(), 0600)
}

// Find a section by section title
func (f *configFile) findSectionByTitle(sectionTitle string) *configSection {
	for _, section := range f.sections {
		if section.title == sectionTitle {
			return section
		}
	}
	return nil
}

// Find a section by section title, and a key/value pair inside it, for example (Peer, PublicKey, 12345...)
func (f *configFile) findSectionByKeyValue(sectionTitle, key, value string) *configSection {
	for _, section := range f.sections {
		if section.title == sectionTitle {
			for _, line := range section.lines {
				if line.key == key && line.value == value {
					return section
				}
			}
		}
	}
	return nil
}

// Add a new section
func (f *configFile) addSection(title string) *configSection {
	section := &configSection{
		title: title,
	}
	f.sections = append(f.sections, section)
	return section
}

// Set a value in a section
func (s *configSection) set(key, value string) {
	for i := range s.lines {
		if s.lines[i].key == key {
			s.lines[i].value = value
			return
		}
	}
	s.lines = append(s.lines, configLine{
		key:   key,
		value: value,
	})
}

// Get a value by key
func (s *configSection) get(key string) *string {
	for i := range s.lines {
		if s.lines[i].key == key {
			return &s.lines[i].value
		}
	}
	return nil
}

// Returns true if the config line is non-empty, and the first non-whitespace character is not #.
// In other words, return false if the line is empty or if the line is a comment.
func hasContent(line string) bool {
	return len(line) > 0 && line[0] != '#'
}

func init() {
	regexpSection = regexp.MustCompile(`^\[([^\]]+)\]$`)
	regexpValue = regexp.MustCompile(`([^=]+)=(.+)`)
}
