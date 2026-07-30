package ci

import (
	"io"
	"testing"
)

type TestingTarBuilder struct {}

func (tb *TestingTarBuilder) TarWorkspace(pw *io.PipeWriter, path string) error {
	return nil
}

func TestCleanOldImages(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		im      ImageManager
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := CleanOldImages(t.Context(), tt.im)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CleanOldImages() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CleanOldImages() succeeded unexpectedly")
			}
		})
	}
}

func Test_buildImage(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		im      ImageManager
		wsName  string
		sha     string
		srcPath string
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := buildImage(t.Context(), tt.im, tt.wsName, tt.sha, tt.srcPath, &TestingTarBuilder{})
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("buildImage() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("buildImage() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("buildImage() = %v, want %v", got, tt.want)
			}
		})
	}
}
