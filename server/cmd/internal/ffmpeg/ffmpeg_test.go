package ffmpeg

import "testing"

// Capabilities() hands out a copy: a caller that edits the maps it gets back
// must not change what the next caller sees, and cloning must not turn an
// unprobed instance's nil maps into empty ones.
func TestCapabilitiesReturnsIndependentSnapshot(t *testing.T) {
	original := Capabilities{
		Probed:         true,
		Encoders:       map[string]bool{"libx264": true},
		Filters:        map[string]bool{"scale": true},
		HWAccels:       map[string]bool{"cuda": true},
		CLIOptions:     map[string]bool{"readrate": true},
		FilterOptions:  map[string]map[string]bool{"scale_cuda": {"format": true}},
		EncoderOptions: map[string]map[string]bool{"h264_qsv": {"preset": true}},
		MuxerFlags:     map[string]map[string]bool{"hls": {"temp_file": true}},
	}
	f := &ffmpeg{capabilities: original}

	first := f.Capabilities()
	delete(first.Encoders, "libx264")
	delete(first.Filters, "scale")
	delete(first.HWAccels, "cuda")
	delete(first.CLIOptions, "readrate")
	delete(first.FilterOptions["scale_cuda"], "format")
	delete(first.EncoderOptions["h264_qsv"], "preset")
	delete(first.MuxerFlags["hls"], "temp_file")

	second := f.Capabilities()
	if !second.Encoders["libx264"] || !second.Filters["scale"] || !second.HWAccels["cuda"] {
		t.Fatalf("top-level maps were mutated through snapshot: %#v", second)
	}
	if !second.CLIOptions["readrate"] || !second.FilterOptions["scale_cuda"]["format"] {
		t.Fatalf("filter/CLI maps were mutated through snapshot: %#v", second)
	}
	if !second.EncoderOptions["h264_qsv"]["preset"] || !second.MuxerFlags["hls"]["temp_file"] {
		t.Fatalf("nested encoder/muxer maps were mutated through snapshot: %#v", second)
	}

	empty := (&ffmpeg{}).Capabilities()
	if empty.Encoders != nil || empty.Filters != nil || empty.HWAccels != nil || empty.CLIOptions != nil {
		t.Fatalf("nil top-level maps changed while cloning: %#v", empty)
	}
	if empty.FilterOptions != nil || empty.EncoderOptions != nil || empty.MuxerFlags != nil {
		t.Fatalf("nil nested maps changed while cloning: %#v", empty)
	}
}
