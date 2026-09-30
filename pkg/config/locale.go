package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DateOrder decides whether an absolute date shows the day or the month first.
type DateOrder string

const (
	DateOrderAuto DateOrder = "auto" // follow the system locale
	DateOrderDMY  DateOrder = "dmy"
	DateOrderMDY  DateOrder = "mdy"
)

func (o *DateOrder) UnmarshalYAML(node *yaml.Node) error {
	switch v := DateOrder(node.Value); v {
	case "", DateOrderAuto, DateOrderDMY, DateOrderMDY:
		*o = v
		return nil
	default:
		return fmt.Errorf("invalid ui.date_order %q: want auto, dmy or mdy", node.Value)
	}
}

// MonthFirst reports whether dates show the month before the day. locale is
// called only for auto, because finding the locale on macOS starts a process.
func (o DateOrder) MonthFirst(locale func() string) bool {
	switch o {
	case DateOrderDMY:
		return false
	case DateOrderMDY:
		return true
	default:
		return LocaleMonthFirst(locale())
	}
}

// monthFirstRegions are the regions whose short numeric dates put the month
// first. Everywhere else, and for a locale with no region, the day comes first.
var monthFirstRegions = map[string]bool{
	"US": true, "PR": true, "GU": true, "VI": true, "AS": true, "MP": true, "UM": true,
	"PH": true, "CA": true, "FM": true, "MH": true, "PW": true,
}

// LocaleMonthFirst reports whether a POSIX or BCP 47 locale name ("en_US.UTF-8",
// "en-US", "zh_Hans_CN") belongs to a month-first region. A macOS region
// override ("en_US@rg=nlzzzz") names the region whose formats apply.
func LocaleMonthFirst(locale string) bool {
	return monthFirstRegions[localeRegion(locale)]
}

func localeRegion(locale string) string {
	name, modifier, _ := strings.Cut(locale, "@")
	for _, part := range strings.Split(modifier, ";") {
		if rg, ok := strings.CutPrefix(part, "rg="); ok && len(rg) >= 2 {
			return strings.ToUpper(rg[:2])
		}
	}
	name, _, _ = strings.Cut(name, ".")
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' })
	for _, part := range parts[min(1, len(parts)):] {
		if len(part) == 2 {
			return strings.ToUpper(part)
		}
	}
	return ""
}

// SystemLocale returns the locale whose date conventions the user chose.
func SystemLocale() string {
	return pickLocale(appleLocale(), os.Getenv)
}

// pickLocale prefers the macOS region setting over the environment: terminals
// commonly export LC_ALL=en_US.UTF-8 whatever region the user picked in System
// Settings, so the environment on a Mac often names the wrong region.
func pickLocale(apple string, getenv func(string) string) string {
	if apple != "" {
		return apple
	}
	for _, key := range []string{"LC_ALL", "LC_TIME", "LANG"} {
		if v := getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func appleLocale() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
