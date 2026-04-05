package model

type ClipItem struct {
	Title string  `json:"title"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type ClipRequest struct {
	VideoPath string     `json:"video_path"`
	Clips     []ClipItem `json:"clips"`
	OutputDir string     `json:"output_dir,omitempty"`
}

type ClipResult struct {
	Title    string  `json:"title"`
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Duration float64 `json:"duration"`
	FilePath string  `json:"file_path"`
}

type ClipResponse struct {
	Outputs []ClipResult `json:"outputs"`
}
