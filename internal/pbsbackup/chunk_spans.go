package pbsbackup

// fileChunkSpan is a slice of a PBS chunk as it appears in the pxar byte stream.
type fileChunkSpan struct {
	Digest  string `json:"digest"`
	Len     int    `json:"len"`
	Partial bool   `json:"partial,omitempty"`
}

// spansReusableForFastReuse reports whether cached spans may be replayed without
// re-reading the file. Partial spans (file boundary inside a CDC chunk) must not
// be reused — PBS references the full chunk blob by digest.
func spansReusableForFastReuse(spans []fileChunkSpan) bool {
	if len(spans) == 0 {
		return false
	}
	for _, sp := range spans {
		if sp.Partial || sp.Len <= 0 || sp.Digest == "" {
			return false
		}
	}
	return true
}

func chunkSpansForRange(records []didxRecord, start, end uint64) []fileChunkSpan {
	if end <= start || len(records) == 0 {
		return nil
	}
	var prev uint64
	spans := make([]fileChunkSpan, 0, 4)
	for _, r := range records {
		chunkEnd := r.offset
		if chunkEnd <= start {
			prev = chunkEnd
			continue
		}
		if prev >= end {
			break
		}
		useStart := prev
		if useStart < start {
			useStart = start
		}
		useEnd := chunkEnd
		if useEnd > end {
			useEnd = end
		}
		if useStart < useEnd {
			chunkStart := prev
			spans = append(spans, fileChunkSpan{
				Digest:  r.digest,
				Len:     int(useEnd - useStart),
				Partial: useStart > chunkStart || useEnd < chunkEnd,
			})
		}
		prev = chunkEnd
	}
	return spans
}

func chunkSpansFromAssignments(assignments []string, offsets []uint64, streamEnd, start, end uint64) []fileChunkSpan {
	if end <= start || len(assignments) == 0 || len(offsets) != len(assignments) {
		return nil
	}
	spans := make([]fileChunkSpan, 0, 4)
	for i := range assignments {
		chunkStart := offsets[i]
		chunkEnd := streamEnd
		if i+1 < len(offsets) {
			chunkEnd = offsets[i+1]
		}
		if chunkEnd <= start || chunkStart >= end {
			continue
		}
		useStart := chunkStart
		if useStart < start {
			useStart = start
		}
		useEnd := chunkEnd
		if useEnd > end {
			useEnd = end
		}
		if useStart < useEnd {
			spans = append(spans, fileChunkSpan{
				Digest:  assignments[i],
				Len:     int(useEnd - useStart),
				Partial: useStart > chunkStart || useEnd < chunkEnd,
			})
		}
	}
	return spans
}
