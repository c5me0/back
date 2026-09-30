package models

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/neko-sc/ent/dialect/sql"
)

const (
	DefaultLimit = 25
	MaxLimit     = 100
)

const cursorSize = 8 + 16

// Cursor points at the last row of a page ordered by (created_at DESC, id DESC).
// On the wire it is base64url(created_at UnixNano as 8 big-endian bytes + 16 UUID bytes).
type Cursor struct { //nolint:recvcheck // decoding requires a pointer receiver
	Time time.Time
	ID   uuid.UUID
}

func (c Cursor) MarshalText() ([]byte, error) {
	raw := binary.BigEndian.AppendUint64(make([]byte, 0, cursorSize), uint64(c.Time.UnixNano()))
	return base64.RawURLEncoding.AppendEncode(nil, append(raw, c.ID[:]...)), nil
}

func (c *Cursor) UnmarshalText(text []byte) error {
	raw, err := base64.RawURLEncoding.DecodeString(string(text))
	if err != nil || len(raw) != cursorSize {
		return errors.New("invalid cursor")
	}

	//nolint:gosec // G115: two's-complement round-trip through uint64 is intended
	c.Time = time.Unix(0, int64(binary.BigEndian.Uint64(raw))).UTC()
	c.ID = uuid.UUID(raw[8:])
	return nil
}

// Query is the pagination part of a list request. Embed it in the request struct.
type Query struct {
	Cursor *Cursor `query:"cursor"`
	Limit  *int32  `query:"limit"`
}

func (q Query) Validate() error {
	return validation.ValidateStruct(&q,
		validation.Field(&q.Limit, validation.Min(int32(1)), validation.Max(int32(MaxLimit))),
	)
}

// PageSize returns the requested limit, DefaultLimit when absent.
func (q Query) PageSize() int {
	if q.Limit == nil {
		return DefaultLimit
	}
	return int(*q.Limit)
}

// Modify applies the cursor, the (created_at DESC, id DESC) order and a PageSize()+1 limit
// to a query over a table with created_at and id columns: `query.Modify(req.Modify).All(ctx)`.
func (q Query) Modify(selector *sql.Selector) {
	createdAt, id := selector.C("created_at"), selector.C("id")
	if q.Cursor != nil {
		selector.Where(sql.CompositeLT([]string{createdAt, id}, q.Cursor.Time, q.Cursor.ID))
	}
	selector.OrderBy(sql.Desc(createdAt), sql.Desc(id)).Limit(q.PageSize() + 1)
}

type Page[T any] struct {
	Items      []T     `json:"items"`
	HasMore    bool    `json:"has_more"`
	NextCursor *Cursor `json:"next_cursor"`
}

// Paginate maps rows fetched with Query.Modify (limit+1 rows) into a page, trimming the look-ahead row.
func Paginate[E, T any](rows []E, limit int, toItem func(E) T, cursorOf func(E) Cursor) Page[T] {
	page := Page[T]{Items: make([]T, 0, min(len(rows), limit))}
	if len(rows) > limit {
		rows = rows[:limit]
		page.HasMore = true
		page.NextCursor = new(cursorOf(rows[len(rows)-1]))
	}

	for _, row := range rows {
		page.Items = append(page.Items, toItem(row))
	}
	return page
}
