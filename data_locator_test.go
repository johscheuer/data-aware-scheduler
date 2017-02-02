package main

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestParseXattrOutput(t *testing.T) {
	s, err := ioutil.ReadFile("test/xattr_example.txt")
	if err != nil {
		log.Fatalln(err)
	}

	segments := parseXattrSegments(string(s))

	expected_segments := 4
	if len(segments) != expected_segments {
		log.Printf("Expected: %d got %d", expected_segments, len(segments))
		t.FailNow()
	}
}
