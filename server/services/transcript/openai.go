package transcript

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	"cameo/internal/ent"
	"cameo/internal/ent/schema_types/call"
)

const summaryInstructions = `너는 커플의 통화를 기록해 주는 도우미야.
입력은 통화 전사본이야. 각 줄은 "[mm:ss] 이름: 발화" 형식이고, "★ [mm:ss] 하이라이트" 줄은 통화 중 한 사람이 그 순간을 기억하고 싶어 표시한 시점이야.
다음 두 가지를 만들어 JSON으로만 출력해.
- title: 이 통화를 따뜻하고 간결하게 담은 제목. 40자 이내.
- summary: 2~4문장 요약. ★ 하이라이트 시점 전후에 나눈 이야기를 중심으로 강조해.
제목과 요약은 전사본과 같은 언어로 쓰고, 전사본에 없는 내용은 지어내지 마.`

var summarySchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"title":   map[string]any{"type": "string"},
		"summary": map[string]any{"type": "string"},
	},
	"required":             []string{"title", "summary"},
	"additionalProperties": false,
}

type summary struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// transcribe returns the speech segments of one participant's track, timed from the call start.
func (s *Service) transcribe(ctx context.Context, path string, speakerID uuid.UUID) ([]call.Segment, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open track: %w", err)
	}
	defer file.Close()

	params := openai.AudioTranscriptionNewParams{
		File:                   file,
		Model:                  s.cfg.OpenAI.TranscriptionModel,
		ResponseFormat:         openai.AudioResponseFormatVerboseJSON,
		TimestampGranularities: []string{"segment"},
	}
	if s.cfg.OpenAI.Language != "" {
		params.Language = openai.String(s.cfg.OpenAI.Language)
	}
	res, err := s.client.Audio.Transcriptions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create transcription: %w", err)
	}

	var verbose struct {
		Segments []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}
	if err = json.Unmarshal([]byte(res.RawJSON()), &verbose); err != nil {
		return nil, fmt.Errorf("parse transcription: %w", err)
	}

	segments := make([]call.Segment, 0, len(verbose.Segments))
	for _, seg := range verbose.Segments {
		if text := strings.TrimSpace(seg.Text); text != "" {
			segments = append(segments, call.Segment{SpeakerUserID: speakerID, Start: seg.Start, End: seg.End, Text: text})
		}
	}
	return segments, nil
}

// summarize asks the summary model for a title and a summary of the transcript, emphasising the highlighted moments.
func (s *Service) summarize(ctx context.Context, row *ent.Call, users []*ent.User, segments []call.Segment, highlights []*ent.CallHighlight) (summary, error) {
	names := map[uuid.UUID]string{row.CallerID: "발신자", row.CalleeID: "수신자"}
	for _, user := range users {
		if user.DisplayName != nil && *user.DisplayName != "" {
			names[user.ID] = *user.DisplayName
		}
	}

	var input strings.Builder
	next := 0
	for _, seg := range segments {
		for ; next < len(highlights) && highlights[next].OffsetSeconds <= seg.Start; next++ {
			fmt.Fprintf(&input, "★ [%s] 하이라이트\n", timestamp(highlights[next].OffsetSeconds))
		}
		fmt.Fprintf(&input, "[%s] %s: %s\n", timestamp(seg.Start), names[seg.SpeakerUserID], seg.Text)
	}
	for _, highlight := range highlights[next:] {
		fmt.Fprintf(&input, "★ [%s] 하이라이트\n", timestamp(highlight.OffsetSeconds))
	}

	res, err := s.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        s.cfg.OpenAI.SummaryModel,
		Instructions: openai.String(summaryInstructions),
		Input:        responses.ResponseNewParamsInputUnion{OfString: openai.String(input.String())},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name:   "call_summary",
					Schema: summarySchema,
					Strict: openai.Bool(true),
				},
			},
		},
	})
	if err != nil {
		return summary{}, fmt.Errorf("create response: %w", err)
	}

	var result summary
	if err = json.Unmarshal([]byte(res.OutputText()), &result); err != nil {
		return summary{}, fmt.Errorf("parse summary: %w", err)
	}
	return result, nil
}

func timestamp(seconds float64) string {
	return fmt.Sprintf("%02d:%02d", int(seconds)/60, int(seconds)%60)
}
