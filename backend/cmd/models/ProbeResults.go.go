package models

type AudioProbe struct {
	Streams []struct {
		Index         int    `json:"index"`
		Codec         string `json:"codec_name"`
		Channels      int    `json:"channels"`
		ChannelLayout string `json:"channel_layout"`
		Profile       string `json:"profile"`

		Tags struct {
			BitRate  string `json:"BaPS"`
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

type SubtitlesProbe struct {
	Streams []struct {
		Index int    `json:"index"`
		Codec string `json:"codec_name"`
		Tags  struct {
			Language string `json:"language"`
		} `json:"tags"`
	} `json:"streams"`
}

type VideoProbe struct {
	Streams []struct {
		Title          string `json:"codec_long_name"`
		Index          uint   `json:"index"`
		BitDepth       string `json:"pix_fmt"`
		CodecName      string `json:"codec_name"`
		Width          uint   `json:"width"`
		Height         uint   `json:"height"`
		CodedHeight    uint   `json:"coded_height"`
		CodedWidth     uint   `json:"coded_heig"`
		ColorSpace     string `json:"color_space"`
		ColorPrimaries string `json:"color_primaries"`
		AspectRatio    string `json:"display_aspect_ratio"`
		FrameRate      string `json:"r_frame_rate"`
		AvgFrameRate   string `json:"avg_frame_rate"`

		Tags struct {
			BitRate        string `json:"BPS"`
			Duration       string `json:"DURATION"`
			NumberOfFrames string `json:"NUMBER_OF_FRAMES"`
			NumberOfBytes  string `json:"NUMBER_OF_BYTES"`
		} `json:"tags"`
	} `json:"streams"`
}

type ChaptersProbe struct {
	Chapters []struct {
		TimeBase  string `json:"time_base"`
		Start     uint   `json:"start"`
		StartTime string `json:"start_time"`
		End       uint   `json:"end"`
		EndTime   string `json:"end_time"`

		Tags struct {
			Title string `json:"title"`
		} `json:"tags"`
	} `json:"chapters"`
}
