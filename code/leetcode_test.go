package code

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/config"
	"github.com/stretchr/testify/assert"
)

//go:embed leetcode-testdata/leetcode-responses/question-submission-list-response.json
var questionSubmissionListResponse []byte

//go:embed leetcode-testdata/leetcode-responses/submission-details-response.json
var submissionDetailsResponse []byte

//go:embed leetcode-testdata/leetcode-responses/user-progress-question-list-response.json
var userProgressQuestionListResponse []byte

var (
	submissionListCalled           = false
	submissionDetailsCalled        = false
	userProgressQuestionListCalled = false
)

var (
	lc             leetcode
	currentHandler func(w http.ResponseWriter, reqBody string)
)

func TestMain(m *testing.M) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ := io.ReadAll(r.Body)
		currentHandler(w, string(reqBody))
	}))
	testUrl := "http://" + server.Listener.Addr().String()
	cfg := config.Config{LcCookie: "COOKIE", RepoUrl: "REPO_URL"}
	lc = NewLeetCode(cfg, testUrl)
	m.Run()
}

func TestFetchSubmissions(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			userProgressQuestionListCalled = true
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Fatal("Couldn't write userProgressQuestionListResponse to response correctly")
			}
		}
		if strings.Contains(reqBody, "submissionList") {
			submissionListCalled = true
			_, err := w.Write(questionSubmissionListResponse)
			if err != nil {
				t.Fatal("Couldn't write questionSubmissionListResponse to response correctly")
			}
		}
		if strings.Contains(reqBody, "submissionDetails") {
			submissionDetailsCalled = true
			_, err := w.Write(submissionDetailsResponse)
			if err != nil {
				t.Fatal("Couldn't write submissionDetailsResponse to response correctly")
			}
		}
	}

	// When
	res, _ := lc.FetchSubmissions()
	submission := res[0]

	// Then
	assert.Equal(t, submission.Id, "128")
	assert.Equal(t, submission.Lang, "golang")
	assert.Equal(t, submission.Title, "Longest Consecutive Sequence")
	assert.True(t, userProgressQuestionListCalled)
	assert.True(t, submissionListCalled)
	assert.True(t, submissionDetailsCalled)
}

func TestFetchSubmissionsShouldReturnErrorWhenFetchQuestionsFails(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			panic("panicing so the method fetchSubmissionCode fails")
		}
	}

	// When
	_, err := lc.FetchSubmissions()

	// Then
	assert.Error(t, err)
}

func TestFetchSubmissionsShouldReturnErrorWhenFetchSubmissionOverviewFails(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "submissionList") {
			panic("panicing so the method fetchSubmissionCode fails")
		}
	}

	// When
	_, err := lc.FetchSubmissions()

	// Then
	assert.Error(t, err)
}

func TestFetchSubmissionsShouldReturnErrorWhenFetchSubmissionCodeFails(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "submissionList") {
			_, err := w.Write(questionSubmissionListResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "submissionDetails") {
			panic("panicing so the method fetchSubmissionCode fails")
		}
	}

	// When
	_, err := lc.FetchSubmissions()

	// Then
	assert.Error(t, err)
}

func TestFetchSubmissionsShouldRetryAndFailWithEmptySubmissionDataDetails(t *testing.T) {
	// Given
	attemptCount := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "submissionDetails") {
			response := RequestBody[lcSubmissionDetailsData]{
				Data: lcSubmissionDetailsData{
					Details: nil,
				},
			}
			json.NewEncoder(w).Encode(response)
			attemptCount++
		}
	}

	// When
	code, err := lc.fetchSubmissionCode("123", 0)

	// Then
	assert.Error(t, err)
	assert.Empty(t, code)
	assert.Equal(t, maxRetry+1, attemptCount)
}

func TestFetchSubmissionsShouldRetryAndSucceedWithValidCode(t *testing.T) {
	// Given
	attemptCount := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "submissionDetails") {
			attemptCount++
			response := RequestBody[lcSubmissionDetailsData]{
				Data: lcSubmissionDetailsData{
					Details: &lcSubmissionDetails{
						Code: "",
					},
				},
			}
			if attemptCount > 3 {
				response.Data.Details.Code = "valid code"
			}
			json.NewEncoder(w).Encode(response)
		}
	}

	// When
	code, err := lc.fetchSubmissionCode("123", 0)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "valid code", code)
	assert.Equal(t, 4, attemptCount)
}

func TestFetchSubmissionsShouldRetryAndFailWithEmptyCode(t *testing.T) {
	// Given
	attemptCount := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "submissionDetails") {
			response := RequestBody[lcSubmissionDetailsData]{
				Data: lcSubmissionDetailsData{
					Details: &lcSubmissionDetails{
						Code: "",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			attemptCount++
		}
	}

	// When
	code, err := lc.fetchSubmissionCode("123", 0)

	// Then
	assert.Error(t, err)
	assert.Empty(t, code)
	assert.Equal(t, maxRetry+1, attemptCount)
}

func TestFetchSubmissionsSinceSkipsOldQuestions(t *testing.T) {
	// Given
	submissionOverviewCalled := false
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, _ = w.Write(userProgressQuestionListResponse)
		}
		if strings.Contains(reqBody, "submissionList") {
			submissionOverviewCalled = true
			_, _ = w.Write(questionSubmissionListResponse)
		}
	}

	// Question 128 has lastSubmittedAt 2024-12-28T17:25:31+00:00
	// Cutoff is set after this date
	since := time.Date(2024, 12, 29, 0, 0, 0, 0, time.UTC)

	// When
	res, err := lc.FetchSubmissionsSince(&since)

	// Then
	assert.NoError(t, err)
	assert.Empty(t, res)
	assert.False(t, submissionOverviewCalled, "Should not make API calls for already-synced questions")
}

func TestFetchSubmissionsSinceIncludesNewQuestions(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, _ = w.Write(userProgressQuestionListResponse)
		}
		if strings.Contains(reqBody, "submissionList") {
			_, _ = w.Write(questionSubmissionListResponse)
		}
		if strings.Contains(reqBody, "submissionDetails") {
			_, _ = w.Write(submissionDetailsResponse)
		}
	}

	// Question 128 has lastSubmittedAt 2024-12-28T17:25:31+00:00
	// Cutoff is set before this date
	since := time.Date(2024, 12, 27, 0, 0, 0, 0, time.UTC)

	// When
	res, err := lc.FetchSubmissionsSince(&since)

	// Then
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "128", res[0].Id)
}
