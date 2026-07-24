package app

import (
	"errors"
	"io"
	"strings"

	"github.com/abema/go-mp4"
	"github.com/at-wat/ebml-go"
	"github.com/at-wat/ebml-go/webm"
)

var errInvalidCommunityVideo = errors.New("视频容器无效或不包含可播放的视频轨道")

func validateCommunityVideo(file io.ReadSeeker, mimeType string, expectedSize int64) error {
	actualSize, err := file.Seek(0, io.SeekEnd)
	if err != nil || actualSize != expectedSize || actualSize < 1 || actualSize > maxCommunityVideoUpload {
		return errInvalidCommunityVideo
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errInvalidCommunityVideo
	}
	switch mimeType {
	case "video/mp4":
		return validateMP4Video(file)
	case "video/webm":
		return validateWebMVideo(file)
	default:
		return errInvalidCommunityVideo
	}
}

func validateMP4Video(file io.ReadSeeker) error {
	boxes, err := mp4.ExtractBoxes(file, nil, []mp4.BoxPath{
		{mp4.BoxTypeFtyp()},
		{mp4.BoxTypeMoov()},
		{mp4.BoxTypeMoov(), mp4.BoxTypeTrak()},
		{mp4.BoxTypeMdat()},
	})
	if err != nil {
		return errInvalidCommunityVideo
	}
	hasFileType, hasMovie, hasSampleData := false, false, false
	tracks := make([]*mp4.BoxInfo, 0)
	for _, box := range boxes {
		switch box.Type {
		case mp4.BoxTypeFtyp():
			hasFileType = true
		case mp4.BoxTypeMoov():
			hasMovie = true
		case mp4.BoxTypeTrak():
			tracks = append(tracks, box)
		case mp4.BoxTypeMdat():
			hasSampleData = hasSampleData || box.Size > box.HeaderSize
		}
	}
	if !hasFileType || !hasMovie || !hasSampleData {
		return errInvalidCommunityVideo
	}
	for _, track := range tracks {
		if mp4TrackHasVideoSamples(file, track) {
			return nil
		}
	}
	return errInvalidCommunityVideo
}

func mp4TrackHasVideoSamples(file io.ReadSeeker, track *mp4.BoxInfo) bool {
	boxes, err := mp4.ExtractBoxesWithPayload(file, track, []mp4.BoxPath{
		{mp4.BoxTypeMdia(), mp4.BoxTypeHdlr()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsz()},
	})
	if err != nil {
		return false
	}
	hasVideoHandler, hasSamples := false, false
	for _, box := range boxes {
		switch payload := box.Payload.(type) {
		case *mp4.Hdlr:
			hasVideoHandler = string(payload.HandlerType[:]) == "vide"
		case *mp4.Stsz:
			hasSamples = payload.SampleCount > 0 && mp4SamplesHaveData(payload)
		}
	}
	return hasVideoHandler && hasSamples
}

func mp4SamplesHaveData(samples *mp4.Stsz) bool {
	if samples.SampleSize > 0 {
		return true
	}
	for _, size := range samples.EntrySize {
		if size > 0 {
			return true
		}
	}
	return false
}

func validateWebMVideo(file io.Reader) error {
	var container struct {
		Header  webm.EBMLHeader `ebml:"EBML"`
		Segment webm.Segment    `ebml:"Segment"`
	}
	if err := ebml.Unmarshal(file, &container); err != nil || !strings.EqualFold(container.Header.DocType, "webm") {
		return errInvalidCommunityVideo
	}
	videoTracks := make(map[uint64]struct{})
	for _, track := range container.Segment.Tracks.TrackEntry {
		if track.TrackType == 1 && track.TrackNumber > 0 && track.Video != nil && track.Video.PixelWidth > 0 && track.Video.PixelHeight > 0 && strings.HasPrefix(track.CodecID, "V_") {
			videoTracks[track.TrackNumber] = struct{}{}
		}
	}
	for _, cluster := range container.Segment.Cluster {
		for _, block := range cluster.SimpleBlock {
			if _, ok := videoTracks[block.TrackNumber]; ok && blockHasData(block) {
				return nil
			}
		}
		for _, group := range cluster.BlockGroup {
			if _, ok := videoTracks[group.Block.TrackNumber]; ok && blockHasData(group.Block) {
				return nil
			}
		}
	}
	return errInvalidCommunityVideo
}

func blockHasData(block ebml.Block) bool {
	for _, frame := range block.Data {
		if len(frame) > 0 {
			return true
		}
	}
	return false
}
