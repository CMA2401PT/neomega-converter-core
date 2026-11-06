package export

import (
	"errors"
	"io/fs"
	"testing"

	"celler"
	java "celler/java"
	"celler/java/region"
)

// 后面的任务即使先算完，严格模式也只写入坏柱之前的前缀；普通模式继续写后面的柱子。
func TestParallelStrictFailureCommitsOnlyEarlierChunks(t *testing.T) {
	source, err := java.OpenDirectory(t.TempDir(), java.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	for x := range int32(3) {
		pos := celler.ChunkPos{X: x}
		if x == 1 {
			if err := source.WriteRaw(java.Overworld, java.Terrain, pos, region.Record{Compression: region.Uncompressed, Data: []byte{10}}); err != nil {
				t.Fatal(err)
			}
		} else {
			chunk := javaChestChunk(t)
			chunk.NBT.Value["xPos"] = x
			if err := source.WriteChunk(java.Overworld, chunk); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, strict := range []bool{false, true} {
		target := openBedrock(t)
		report, err := ConvertWorld(source, target, ConvertOptions{Strict: strict, Workers: 4, NewWorld: true})
		if (err != nil) != strict || len(report.FailedChunks) != 1 || report.FailedChunks[0].Pos.X != 1 {
			t.Fatal(report, err)
		}
		want := uint64(2)
		if strict {
			want = 1
		}
		if report.Chunks[0] != want {
			t.Fatal(report)
		}
		if _, err := target.ReadChunk(0, celler.ChunkPos{}); err != nil {
			t.Fatal(err)
		}
		_, err = target.ReadChunk(0, celler.ChunkPos{X: 2})
		if strict && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		if !strict && err != nil {
			t.Fatal(err)
		}
	}
}

// 最后一批提交失败时，报告不能把仍在内存中的柱子算作已写出。
func TestFailedBatchIsNotCounted(t *testing.T) {
	source, err := java.OpenDirectory(t.TempDir(), java.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.WriteChunk(java.Overworld, javaChestChunk(t)); err != nil {
		t.Fatal(err)
	}
	target := openBedrock(t)
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	report, err := ConvertWorld(source, target, ConvertOptions{Workers: 4, NewWorld: true})
	if !errors.Is(err, fs.ErrClosed) || report.Chunks[0] != 0 || report.Entities != 0 {
		t.Fatal(report, err)
	}
}
