// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package sqlc_model

type Url struct {
	ID        string
	Value     string
	Via       string
	Hops      int64
	Status    string
	Timestamp int64
}
