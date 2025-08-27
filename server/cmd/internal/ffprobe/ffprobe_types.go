package ffprobe

type FfprobeInterface interface {
	GetTrackMetadata(trackPath string) (*TrackFfprobeResult, error)
}

type Ffprobe struct {
	bin string
}

type TrackFfprobeResult struct {
	Streams []struct {
		CodecName     string `json:"codec_name"`
		CodecType     string `json:"codec_type"`
		Profile       string `json:"profile"`
		SampleRate    string `json:"sample_rate"`
		Channels      int    `json:"channels"`
		ChannelLayout string `json:"channel_layout"`
		BitRate       int    `json:"bit_rate"`

		Tags StreamTag `json:"tags"`
	} `json:"streams"`

	Format struct {
		FileName string `json:"file_name"`
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`

		Tags struct {
			Title        string `json:"title"`
			Artist       string `json:"artist"`
			AlbumArtist  string `json:"album_artist"`
			Composer     string `json:"composer"`
			Album        string `json:"album"`
			Genre        string `json:"genre"`
			Track        string `json:"track"`
			Disc         string `json:"disc"`
			Date         string `json:"date"`
			Copyright    string `json:"copyright"`
			PurchaseDate string `json:"purchase_date"`
			SortName     string `json:"sort_name"`
			SortAlbum    string `json:"sort_album"`
			SortArtist   string `json:"sort_artist"`
		} `json:"tagsd"`
	} `json:"format"`
}

type MovieMetadataResult struct {
	Streams []struct {
		Index          int    `json:"index"`
		CodecName      string `json:"codec_name"`
		CodecType      string `json:"codec_type"`
		Profile        string `json:"profile"`
		Height         uint   `json:"height"`
		Width          uint   `json:"width"`
		CodedHeight    uint   `json:"coded_height"`
		CodedWidth     uint   `json:"coded_width"`
		AspectRatio    string `json:"display_aspect_ratio"`
		Level          int    `json:"level"`
		AvgFrameRate   string `json:"avg_frame_rate"`
		FrameRate      string `json:"r_frame_rate"`
		BitDepth       string `json:"bits_per_raw_sample"`
		BitRate        string `json:"bit_rate"`
		ColorRange     string `json:"color_range"`
		ColorTransfer  string `json:"color_transfer"`
		ColorPrimaries string `json:"color_primaries"`
		ColorSpace     string `json:"color_space"`
		Channels       int    `json:"channels"`
		ChannelLayout  string `json:"channel_layout"`

		Tags StreamTag `json:"tags"`
	} `json:"streams"`

	Format struct {
		Filename       string `json:"filename"`
		BitRate        string `json:"bit_rate"`
		Size           string `json:"size"`
		Duration       string `json:"duration"`
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"format_long_name"`
	} `json:"format"`

	Chapters Chapter `json:"chapters"`
}

type StreamTag struct {
	Title    string `json:"title"`
	Language string `json:"language"`
}

type Chapter struct {
	StartTime string `json:"start_time"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	EndTime   string `json:"end_time"`

	Tags struct {
		Title string `json:"title"`
	} `json:"tags"`
}
