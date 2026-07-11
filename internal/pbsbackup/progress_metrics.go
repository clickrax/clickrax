package pbsbackup

import "time"

type pbsProgressSample struct {
	bytesNew           int64
	bytesReused        int64
	virtualBytes       int64
	filesDone          int64
	filesTotalEstimate int64
	fastReuseActive    bool
	bytesTotalEstimate int64
}

type pbsProgressRates struct {
	lastBytesNew  int64
	lastProcessed int64
	lastFiles     int64
	lastTick      time.Time
}

func effectiveProcessedBytes(s pbsProgressSample) int64 {
	processed := s.bytesNew + s.bytesReused
	if s.fastReuseActive && s.virtualBytes > processed {
		return s.virtualBytes
	}
	return processed
}

func computePBSSpeedAndETA(s pbsProgressSample, r *pbsProgressRates, now time.Time) (speedBps, etaSec int64) {
	processed := effectiveProcessedBytes(s)

	var uploadSpeed int64
	var processedSpeed int64
	var filesPerSec float64
	if !r.lastTick.IsZero() {
		dt := now.Sub(r.lastTick).Seconds()
		if dt > 0 {
			uploadSpeed = int64(float64(s.bytesNew-r.lastBytesNew) / dt)
			processedSpeed = int64(float64(processed-r.lastProcessed) / dt)
			if s.filesDone > r.lastFiles {
				filesPerSec = float64(s.filesDone-r.lastFiles) / dt
			}
		}
	}
	r.lastBytesNew = s.bytesNew
	r.lastProcessed = processed
	r.lastFiles = s.filesDone
	r.lastTick = now

	speedBps = uploadSpeed
	if processedSpeed > speedBps {
		speedBps = processedSpeed
	}

	if s.bytesTotalEstimate > processed && processedSpeed > 0 {
		etaSec = int64(float64(s.bytesTotalEstimate-processed) / float64(processedSpeed))
	}
	if etaSec == 0 && s.fastReuseActive && s.filesTotalEstimate > s.filesDone && filesPerSec > 0 {
		etaSec = int64(float64(s.filesTotalEstimate-s.filesDone) / filesPerSec)
	}
	return speedBps, etaSec
}

func computePBSTransferPercent(s pbsProgressSample, newChunks, reusedChunks uint64) float64 {
	processed := effectiveProcessedBytes(s)
	pct := 25.0
	if s.fastReuseActive && s.filesTotalEstimate > 0 && s.filesDone > 0 {
		denom := s.filesTotalEstimate
		if s.filesDone > denom {
			denom = s.filesDone
		}
		pct = 25.0 + float64(s.filesDone)/float64(denom)*50.0
	} else if s.bytesTotalEstimate > 0 && processed > 0 {
		pct = 25.0 + float64(processed)/float64(s.bytesTotalEstimate)*50.0
	} else if total := newChunks + reusedChunks; total > 0 {
		pct = 25.0 + float64(newChunks)/float64(total)*50.0
	}
	if pct > 75 {
		pct = 75
	}
	return pct
}
