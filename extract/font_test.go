package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

func spec(name string) pdfsrc.FontSpec { return pdfsrc.FontSpec{Family: name} }

// A font is read by its family and not by the printing it came out of. Lie
// chapters 7 to 9 is set in Knuth's Computer Modern where the volumes read
// before it are set in Latin Modern, so its mathematics italic is called CMMI10
// and not LMMathItalic10, and until the table below had both names 31496 runs of
// variables came through as English words.
func TestClassifyReadsBothFoundries(t *testing.T) {
	for name, want := range map[string]Class{
		"LMMathItalic10":    ClassMath,
		"CMMI10":            ClassMath,
		"CMMIB10":           ClassMath,
		"LMMathSymbols8":    ClassMath,
		"CMSY7":             ClassMath,
		"CMEX10":            ClassMath,
		"LMMathExtension10": ClassMath,
		"RSFS10":            ClassMath,
		"EUFM10":            ClassMath,
		"MSBM10":            ClassMath,
		"LMRoman10":         ClassText,
		"CMR10":             ClassText,
		"CMTI10":            ClassText,
		"LMRomanCaps10":     ClassHead,
		"XYCMAT-Medium":     ClassDiagram,
	} {
		if got := Classify(spec(name), pdfsrc.Span{Text: "x"}); got != want {
			t.Errorf("%s is read as %v, want %v", name, got, want)
		}
	}
}

// A subset tag is six capitals and a plus sign, except where it is not: the
// French Théories spectrales runs four of them together in front of the name
// with no plus anywhere, so a font arrives called
// QnnxqjJsnccdPstrhwMdcrddLMMathItalic10 and reading it by prefix left every
// variable in the volume classed as prose.
func TestFamilyReadsARunOnSubsetTag(t *testing.T) {
	for name, want := range map[string]string{
		"CMEX10":                                 "CMEX",
		"XAEWAV+CMEX10":                          "CMEX",
		"QnnxqjJsnccdPstrhwMdcrddLMMathItalic10": "LMMathItalic",
		"BhpkhsFhmqhpTimesNewRoman":              "TimesNewRoman",
		"HbbykcHvqgbvTimes New Roman":            "Times New Roman",
		"TmkqqjJqmyhcTimesNewRomanItalic":        "TimesNewRoman",
		"AAAAAA+TimesNewRoman,Italic":            "TimesNewRoman",
		"LMRomanCaps10-Regular":                  "LMRomanCaps",
	} {
		if got := family(spec(name)); got != want {
			t.Errorf("family(%q) = %q, want %q", name, got, want)
		}
	}
}

// The longer stem wins, or LMRomanCaps is read as LMRoman and a statement head
// comes through as prose.
func TestFamilyPrefersTheLongerStem(t *testing.T) {
	if got := family(spec("LMRomanCaps10")); got != "LMRomanCaps" {
		t.Errorf("family = %q, want LMRomanCaps", got)
	}
	if got := family(spec("TimesNewRomanPSMT")); got != "TimesNewRomanPSMT" {
		t.Errorf("family = %q, want TimesNewRomanPSMT", got)
	}
}

// A font the table has never been shown is not a class. The run is read as prose
// because there is nothing else to do with it, and the page it is on says so,
// which is the whole difference between a volume that reports 100% clean and a
// volume that is.
func TestKnownFont(t *testing.T) {
	if !KnownFont(spec("XAEWAV+CMEX10")) {
		t.Error("the extension font is not known")
	}
	if KnownFont(spec("Wingdings")) {
		t.Error("a font nothing knows was called known")
	}
}
