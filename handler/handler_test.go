package handler

import (
	"errors"
	"testing"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/code"
	"github.com/ahmed-e-abdulaziz/glsync/git"
	"github.com/ahmed-e-abdulaziz/glsync/mocks/mock_code"
	"github.com/ahmed-e-abdulaziz/glsync/mocks/mock_git"
	"go.uber.org/mock/gomock"
)

func TestExecute(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	// Dec 14 is committed first, Dec 30 is committed second (chronological order)
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().
			Commit(
				[]git.CommitFile{{FolderName: "2 Add Two Numbers", FileName: "2add-two-numbers.go", Code: subs[1].Code}},
				"Code challenge submissions for 2024-12-14 (1 question)\n\n- 2 Add Two Numbers",
				subs[1].LastSubmittedAt.UTC(),
			).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().
			Commit(
				[]git.CommitFile{{FolderName: "1 Two Sum", FileName: "1two-sum.go", Code: subs[0].Code}},
				"Code challenge submissions for 2024-12-30 (1 question)\n\n- 1 Two Sum",
				subs[0].LastSubmittedAt.UTC(),
			).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().Push().Return(nil).Times(1),
	)

	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func TestExecuteShouldGroupMultipleSubmissionsOnSameDayIntoSingleCommit(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	t1 := parseRFC3339("2024-12-28T10:00:00Z")
	t2 := parseRFC3339("2024-12-28T17:30:00Z") // Max timestamp for the day
	t3 := parseRFC3339("2024-12-28T12:15:00Z")

	subs := []code.Submission{
		{Id: "1", Title: "Two Sum", TitleSlug: "two-sum", LastSubmittedAt: t1, Lang: "golang", Code: "code1"},
		{Id: "3", Title: "Longest Substring", TitleSlug: "longest-substring", LastSubmittedAt: t2, Lang: "golang", Code: "code3"},
		{Id: "2", Title: "Add Two Numbers", TitleSlug: "add-two-numbers", LastSubmittedAt: t3, Lang: "golang", Code: "code2"},
	}

	expectedFiles := []git.CommitFile{
		{FolderName: "1 Two Sum", FileName: "1two-sum.go", Code: "code1"},
		{FolderName: "2 Add Two Numbers", FileName: "2add-two-numbers.go", Code: "code2"},
		{FolderName: "3 Longest Substring", FileName: "3longest-substring.go", Code: "code3"},
	}
	expectedMessage := "Code challenge submissions for 2024-12-28 (3 questions)\n\n- 1 Two Sum\n- 2 Add Two Numbers\n- 3 Longest Substring"

	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().
			Commit(expectedFiles, expectedMessage, t2).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().Push().Return(nil).Times(1),
	)

	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func stubSubmissions() []code.Submission {
	subs := []code.Submission{
		{
			Id:              "1",
			Title:           "Two Sum",
			TitleSlug:       "two-sum",
			LastSubmittedAt: parseRFC3339("2024-12-31T00:00:00+02:00"),
			Lang:            "golang",
			Code:            "package main\n",
		},
		{
			Id:              "2",
			Title:           "Add Two Numbers",
			TitleSlug:       "add-two-numbers",
			LastSubmittedAt: parseRFC3339("2024-12-15T00:00:00+02:00"),
			Lang:            "golang",
			Code:            "package main\n",
		},
	}
	return subs
}

func TestExecuteShouldPanicWhenFetchSubmissionFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()
	mockCodeClient.EXPECT().FetchSubmissions().Return(nil, errors.New("mock error")).Times(1)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic although fetch sumbissions failed")
		}
	}()
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func TestExecuteShouldContinueWhenACommitFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().
			Commit(
				[]git.CommitFile{{FolderName: "2 Add Two Numbers", FileName: "2add-two-numbers.go", Code: subs[1].Code}},
				"Code challenge submissions for 2024-12-14 (1 question)\n\n- 2 Add Two Numbers",
				subs[1].LastSubmittedAt.UTC(),
			).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().
			Commit(
				[]git.CommitFile{{FolderName: "1 Two Sum", FileName: "1two-sum.go", Code: subs[0].Code}},
				"Code challenge submissions for 2024-12-30 (1 question)\n\n- 1 Two Sum",
				subs[0].LastSubmittedAt.UTC(),
			).
			Return(errors.New("Second Commit Failed")).
			Times(1),
		mockGitClient.EXPECT().Push().Return(nil).Times(1),
	)
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func TestExecuteShouldPanicWhenPushFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().
			Commit(
				[]git.CommitFile{{FolderName: "2 Add Two Numbers", FileName: "2add-two-numbers.go", Code: subs[1].Code}},
				"Code challenge submissions for 2024-12-14 (1 question)\n\n- 2 Add Two Numbers",
				subs[1].LastSubmittedAt.UTC(),
			).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().
			Commit(
				[]git.CommitFile{{FolderName: "1 Two Sum", FileName: "1two-sum.go", Code: subs[0].Code}},
				"Code challenge submissions for 2024-12-30 (1 question)\n\n- 1 Two Sum",
				subs[0].LastSubmittedAt.UTC(),
			).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().Push().Return(errors.New("Error happened while pushing")).Times(1),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic although git push failed")
		}
	}()
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func initMocks(t *testing.T) (*gomock.Controller, *mock_code.MockCodeClient, *mock_git.MockGitClient) {
	ctrl := gomock.NewController(t)
	mockCodeClient := mock_code.NewMockCodeClient(ctrl)
	mockGitClient := mock_git.NewMockGitClient(ctrl)
	return ctrl, mockCodeClient, mockGitClient
}

func parseRFC3339(timeString string) time.Time {
	timestamp, _ := time.Parse(time.RFC3339, timeString)
	return timestamp
}
