package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type OpenAI struct {
	APIKey             string `json:"api_key"`
	TranscriptionModel string `json:"transcription_model"`
	SummaryModel       string `json:"summary_model"`
	// Language is an optional ISO-639-1 hint for transcription.
	Language string `json:"language"`
}

func (o *OpenAI) Validate() error {
	if o.TranscriptionModel == "" {
		o.TranscriptionModel = "whisper-1"
	}
	if o.SummaryModel == "" {
		o.SummaryModel = "gpt-5-mini"
	}

	return validation.ValidateStruct(o,
		validation.Field(&o.APIKey, validation.Required),
	)
}
