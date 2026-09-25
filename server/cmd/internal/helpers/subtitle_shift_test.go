package helpers

import (
	"strings"
	"testing"
)

func TestShiftWebVTT_RebasesCuesOntoSessionTimeline(t *testing.T) {
	raw := []byte("WEBVTT\n\n" +
		"00:10:05.000 --> 00:10:07.500\nFirst line\n\n" +
		"00:10:09.250 --> 00:10:11.000\nSecond line\n")

	got := string(ShiftWebVTT(raw, 600))

	if !strings.Contains(got, "00:00:05.000 --> 00:00:07.500") {
		t.Fatalf("first cue was not rebased: %s", got)
	}
	if !strings.Contains(got, "00:00:09.250 --> 00:00:11.000") {
		t.Fatalf("second cue was not rebased: %s", got)
	}
	if !strings.HasPrefix(got, "WEBVTT") {
		t.Fatalf("header was not preserved: %s", got)
	}
	if !strings.Contains(got, "First line") || !strings.Contains(got, "Second line") {
		t.Fatalf("cue text was not preserved: %s", got)
	}
}

func TestShiftWebVTT_ZeroOffsetIsUnchanged(t *testing.T) {
	raw := []byte("WEBVTT\n\n00:00:05.000 --> 00:00:07.500\nLine\n")

	got := string(ShiftWebVTT(raw, 0))

	if got != string(raw) {
		t.Fatalf("zero offset must not rewrite the payload, got: %s", got)
	}
}

func TestShiftWebVTT_DropsCuesEntirelyBeforeTheSessionStart(t *testing.T) {
	raw := []byte("WEBVTT\n\n" +
		"00:00:05.000 --> 00:00:07.000\nEarly line\n\n" +
		"00:10:05.000 --> 00:10:07.000\nKept line\n")

	got := string(ShiftWebVTT(raw, 600))

	if strings.Contains(got, "Early line") {
		t.Fatalf("a cue before the session start should be dropped: %s", got)
	}
	if !strings.Contains(got, "Kept line") {
		t.Fatalf("a cue inside the session should survive: %s", got)
	}
}

func TestShiftWebVTT_ClampsCueStraddlingTheStart(t *testing.T) {
	raw := []byte("WEBVTT\n\n00:09:58.000 --> 00:10:03.000\nStraddling\n")

	got := string(ShiftWebVTT(raw, 600))

	if !strings.Contains(got, "00:00:00.000 --> 00:00:03.000") {
		t.Fatalf("straddling cue was not clamped to zero: %s", got)
	}
}

func TestShiftWebVTT_PreservesCueSettingsAndIdentifiers(t *testing.T) {
	raw := []byte("WEBVTT\n\n" +
		"cue-42\n00:10:05.000 --> 00:10:07.000 align:start position:10%\nLine\n")

	got := string(ShiftWebVTT(raw, 600))

	if !strings.Contains(got, "00:00:05.000 --> 00:00:07.000 align:start position:10%") {
		t.Fatalf("cue settings were not preserved: %s", got)
	}
	if !strings.Contains(got, "cue-42") {
		t.Fatalf("cue identifier was not preserved: %s", got)
	}
}

// A dropped cue must take its identifier with it, or the identifier attaches
// itself to the following cue.
func TestShiftWebVTT_DropsIdentifierOfDroppedCue(t *testing.T) {
	raw := []byte("WEBVTT\n\n" +
		"early-cue\n00:00:01.000 --> 00:00:02.000\nEarly\n\n" +
		"kept-cue\n00:10:05.000 --> 00:10:07.000\nKept\n")

	got := string(ShiftWebVTT(raw, 600))

	if strings.Contains(got, "early-cue") {
		t.Fatalf("identifier of a dropped cue was left behind: %s", got)
	}
	if !strings.Contains(got, "kept-cue") {
		t.Fatalf("identifier of a surviving cue was removed: %s", got)
	}
}

func TestShiftWebVTT_AcceptsShortTimestampForm(t *testing.T) {
	raw := []byte("WEBVTT\n\n10:05.000 --> 10:07.000\nLine\n")

	got := string(ShiftWebVTT(raw, 600))

	if !strings.Contains(got, "00:00:05.000 --> 00:00:07.000") {
		t.Fatalf("MM:SS.mmm form was not handled: %s", got)
	}
}

func TestShiftWebVTT_PreservesStructuralBlocks(t *testing.T) {
	raw := []byte("WEBVTT\n\n" +
		"NOTE this is a comment\n\n" +
		"STYLE\n::cue { color: white }\n\n" +
		"00:10:05.000 --> 00:10:07.000\nLine\n")

	got := string(ShiftWebVTT(raw, 600))

	if !strings.Contains(got, "NOTE this is a comment") {
		t.Fatalf("NOTE block was dropped: %s", got)
	}
	if !strings.Contains(got, "::cue { color: white }") {
		t.Fatalf("STYLE block was dropped: %s", got)
	}
}

func TestShiftWebVTT_EmptyInputIsUnchanged(t *testing.T) {
	got := ShiftWebVTT(nil, 600)

	if len(got) != 0 {
		t.Fatalf("empty input must stay empty, got %q", string(got))
	}
}

// Dropping a cue removes the line before its timing only when that line is a
// cue identifier; a header or structural block immediately before it stays.
func TestShiftWebVTT_DroppedCueKeepsStructuralLineBeforeIt(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{name: "header", line: "WEBVTT"},
		{name: "note", line: "NOTE early cue follows"},
		{name: "style", line: "STYLE"},
		{name: "region", line: "REGION id:r1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.line + "\n" +
				"00:00:01.000 --> 00:00:02.000\nEarly\n\n" +
				"00:10:05.000 --> 00:10:07.000\nKept\n")

			got := string(ShiftWebVTT(raw, 600))

			if !strings.HasPrefix(got, tt.line+"\n") {
				t.Fatalf("structural line was dropped with the cue: %s", got)
			}
			if strings.Contains(got, "Early") {
				t.Fatalf("early cue survived: %s", got)
			}
			if !strings.Contains(got, "00:00:05.000 --> 00:00:07.000\nKept") {
				t.Fatalf("kept cue was not rebased: %s", got)
			}
		})
	}
}

func TestShiftWebVTT_FormatsHoursAndRoundsToMilliseconds(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		offset float64
		want   string
	}{
		{
			name:   "an hour or more keeps the hour field",
			raw:    "01:10:05.000 --> 01:10:07.000",
			offset: 600,
			want:   "01:00:05.000 --> 01:00:07.000",
		},
		{
			name:   "sub-millisecond remainders round to the nearest millisecond",
			raw:    "00:10:05.000 --> 00:10:07.000",
			offset: 599.9994,
			want:   "00:00:05.001 --> 00:00:07.001",
		},
		{
			name:   "rounding down does not borrow from the seconds field",
			raw:    "00:00:10.000 --> 00:00:12.000",
			offset: 0.0006,
			want:   "00:00:09.999 --> 00:00:11.999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(ShiftWebVTT([]byte("WEBVTT\n\n"+tt.raw+"\nLine\n"), tt.offset))
			if !strings.Contains(got, tt.want) {
				t.Fatalf("ShiftWebVTT(%q, %v) = %s, want it to contain %q", tt.raw, tt.offset, got, tt.want)
			}
		})
	}
}
