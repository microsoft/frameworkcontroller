// MIT License
//
// Copyright (c) Microsoft Corporation. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE

package common

import (
	"fmt"
	"strings"
	"time"
	"os"
	"gopkg.in/yaml.v2"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"math/rand"
)

func Quote(s string) string {
	return `"` + s + `"`
}

func ReferEnvVar(name string) string {
	return "$(" + name + ")"
}

func PtrString(o string) *string {
	return &o
}

func PtrInt32(o int32) *int32 {
	return &o
}

func NilInt32() *int32 {
	return nil
}

func PtrInt64(o int64) *int64 {
	return &o
}

func PtrFloat64(o float64) *float64 {
	return &o
}

func PtrBool(o bool) *bool {
	return &o
}

func NilBool() *bool {
	return nil
}

func PtrUID(o types.UID) *types.UID {
	return &o
}

func PtrUIDStr(s string) *types.UID {
	return PtrUID(types.UID(s))
}

func PtrNow() *meta.Time {
	now := meta.Now()
	return &now
}

func SecToDuration(sec *int64) time.Duration {
	return time.Duration(*sec) * time.Second
}

func IsTimeout(leftDuration time.Duration) bool {
	// Align with the AddAfter method of the workqueue
	return leftDuration <= 0
}

func CurrentLeftDuration(startTime meta.Time, timeoutSec *int64) time.Duration {
	currentDuration := time.Since(startTime.Time)
	timeoutDuration := SecToDuration(timeoutSec)
	leftDuration := timeoutDuration - currentDuration
	return leftDuration
}

func InitAll() {
	InitLogger()
	InitRandSeed()
}

func InitLogger() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		// Always log with full timestamp, regardless of whether TTY is attached
		DisableTimestamp: false,
		FullTimestamp:    true,
		// Align with k8s.io/apimachinery/pkg/apis/meta/v1.Time
		TimestampFormat: time.RFC3339,
	})

	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stdout)
}

func LogLines(format string, args ...interface{}) {
	lines := strings.Split(fmt.Sprintf(format, args), "\n")
	for _, line := range lines {
		log.Infof(line)
	}
}

func InitRandSeed() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// Rand in range [min, max]
func RandInt64(min int64, max int64) int64 {
	return min + rand.Int63n(max-min+1)
}

func ToYaml(obj interface{}) string {
	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		panic(fmt.Errorf("Failed to marshal Object %#v to YAML: %v", obj, err))
	}
	return string(yamlBytes)
}

func FromYaml(yamlStr string, objAddr interface{}) {
	err := yaml.Unmarshal([]byte(yamlStr), objAddr)
	if err != nil {
		panic(fmt.Errorf("Failed to unmarshal YAML %#v to Object: %v", yamlStr, err))
	}
}

func ToJson(obj interface{}) string {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		panic(fmt.Errorf("Failed to marshal Object %#v to JSON: %v", obj, err))
	}
	return string(jsonBytes)
}

func FromJson(jsonStr string, objAddr interface{}) {
	err := json.Unmarshal([]byte(jsonStr), objAddr)
	if err != nil {
		panic(fmt.Errorf("Failed to unmarshal JSON %#v to Object: %v", jsonStr, err))
	}
}
