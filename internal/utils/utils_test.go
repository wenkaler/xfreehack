package utils

import (
	"reflect"
	"testing"
	"time"
)

func TestParseMonth(t *testing.T) {
	type args struct {
		monthStr string
		yearStr  string
	}
	tests := []struct {
		name    string
		args    args
		want    *time.Time
		wantErr bool
	}{
		{
			name: "normal test",
			args: args{
				monthStr: "январь",
				yearStr:  "2025",
			},
			want:    tmToPtm(time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)),
			wantErr: false,
		},
		{
			name: "failed test",
			args: args{
				monthStr: "фвалдо",
				yearStr:  "2025",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMonth(tt.args.monthStr, tt.args.yearStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMonth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMonth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func tmToPtm(t time.Time) *time.Time {
	return &t
}
