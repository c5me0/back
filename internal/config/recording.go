package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type Recording struct {
	Dir         string `json:"dir"`
	FFmpegPath  string `json:"ffmpeg_path"`
	Concurrency int    `json:"concurrency"`
}

func (r *Recording) Validate() error {
	if r.FFmpegPath == "" {
		r.FFmpegPath = "ffmpeg"
	}
	if r.Concurrency == 0 {
		r.Concurrency = 2
	}

	return validation.ValidateStruct(r,
		validation.Field(&r.Dir, validation.Required),
		validation.Field(&r.Concurrency, validation.Min(1)),
	)
}
