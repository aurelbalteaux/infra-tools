//nolint:testpackage // Test package uses internal package for access to private members
package github

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	gh "github.com/google/go-github/v68/github"
)

type fakeCommentsService struct {
	comments []*gh.IssueComment
	created  []*gh.IssueComment
	edited   map[int64]*gh.IssueComment
	nextID   int64
}

func newFakeCommentsService() *fakeCommentsService {
	return &fakeCommentsService{
		edited: make(map[int64]*gh.IssueComment),
		nextID: 1,
	}
}

func (f *fakeCommentsService) ListComments(_ context.Context, _, _ string, _ int, _ *gh.IssueListCommentsOptions) ([]*gh.IssueComment, *gh.Response, error) {
	return f.comments, &gh.Response{}, nil
}

func (f *fakeCommentsService) CreateComment(_ context.Context, _, _ string, _ int, comment *gh.IssueComment) (*gh.IssueComment, *gh.Response, error) {
	comment.ID = gh.Ptr(f.nextID)
	f.nextID++
	f.created = append(f.created, comment)
	f.comments = append(f.comments, comment)
	return comment, &gh.Response{}, nil
}

func (f *fakeCommentsService) EditComment(_ context.Context, _, _ string, commentID int64, comment *gh.IssueComment) (*gh.IssueComment, *gh.Response, error) {
	f.edited[commentID] = comment
	for i, c := range f.comments {
		if c.GetID() == commentID {
			f.comments[i].Body = comment.Body
		}
	}
	return comment, &gh.Response{}, nil
}

func TestUpsertComment_CreatesNew(t *testing.T) {
	g := NewWithT(t)

	fake := newFakeCommentsService()
	client := &CommentClient{comments: fake, owner: "org", repo: "repo"}

	body := RenderDiffCommentMarker + "\n### Test\nHello"
	err := client.UpsertComment(context.Background(), 42, body)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fake.created).To(HaveLen(1))
	g.Expect(*fake.created[0].Body).To(Equal(body))
}

func TestUpsertComment_UpdatesExisting(t *testing.T) {
	g := NewWithT(t)

	fake := newFakeCommentsService()
	// Pre-existing comment with the marker
	fake.comments = []*gh.IssueComment{
		{ID: gh.Ptr(int64(99)), Body: gh.Ptr(RenderDiffCommentMarker + "\nold content")},
	}

	client := &CommentClient{comments: fake, owner: "org", repo: "repo"}

	body := RenderDiffCommentMarker + "\n### Updated\nNew content"
	err := client.UpsertComment(context.Background(), 42, body)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fake.created).To(BeEmpty()) // no new comment created
	g.Expect(fake.edited).To(HaveKey(int64(99)))
	g.Expect(*fake.edited[99].Body).To(Equal(body))
}

func TestUpsertComment_IgnoresUnrelatedComments(t *testing.T) {
	g := NewWithT(t)

	fake := newFakeCommentsService()
	fake.comments = []*gh.IssueComment{
		{ID: gh.Ptr(int64(1)), Body: gh.Ptr("unrelated comment")},
		{ID: gh.Ptr(int64(2)), Body: gh.Ptr("another comment")},
	}

	client := &CommentClient{comments: fake, owner: "org", repo: "repo"}

	body := RenderDiffCommentMarker + "\nnew"
	err := client.UpsertComment(context.Background(), 42, body)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fake.created).To(HaveLen(1)) // created new, didn't update existing
}

func TestUpsertCommentByMarker_CustomMarker(t *testing.T) {
	g := NewWithT(t)
	const customMarker = "<!-- custom-tool-marker -->"

	fake := newFakeCommentsService()
	client := &CommentClient{
		comments: fake,
		owner:    "test-owner",
		repo:     "test-repo",
	}

	// First call creates comment with custom marker
	body1 := customMarker + "\nNew comment"
	err := client.UpsertCommentByMarker(context.Background(), 123, body1, customMarker)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(fake.comments).To(HaveLen(1))
	g.Expect(*fake.comments[0].Body).To(ContainSubstring(customMarker))

	// Second call updates the same comment
	body2 := customMarker + "\nUpdated comment"
	err = client.UpsertCommentByMarker(context.Background(), 123, body2, customMarker)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(fake.comments).To(HaveLen(1))
	g.Expect(*fake.comments[0].Body).To(ContainSubstring("Updated comment"))
}

func TestUpsertCommentByMarker_EmptyMarkerError(t *testing.T) {
	g := NewWithT(t)

	fake := newFakeCommentsService()
	client := &CommentClient{
		comments: fake,
		owner:    "test-owner",
		repo:     "test-repo",
	}

	err := client.UpsertCommentByMarker(context.Background(), 123, "Comment", "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("marker must not be empty"))
}

func TestUpsertCommentByMarker_MultipleMarkers(t *testing.T) {
	g := NewWithT(t)
	const marker1 = "<!-- tool-1 -->"
	const marker2 = "<!-- tool-2 -->"

	fake := newFakeCommentsService()
	client := &CommentClient{
		comments: fake,
		owner:    "test-owner",
		repo:     "test-repo",
	}

	// Create two comments with different markers
	body1 := marker1 + "\nTool 1 comment"
	err := client.UpsertCommentByMarker(context.Background(), 123, body1, marker1)
	g.Expect(err).NotTo(HaveOccurred())

	body2 := marker2 + "\nTool 2 comment"
	err = client.UpsertCommentByMarker(context.Background(), 123, body2, marker2)
	g.Expect(err).NotTo(HaveOccurred())

	// Should have 2 separate comments
	g.Expect(fake.comments).To(HaveLen(2))

	// Update tool 1's comment - should only affect first comment
	body1Updated := marker1 + "\nTool 1 updated"
	err = client.UpsertCommentByMarker(context.Background(), 123, body1Updated, marker1)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(fake.comments).To(HaveLen(2))

	// Verify only first comment was updated
	foundUpdated := false
	foundUnchanged := false
	for _, c := range fake.comments {
		if *c.Body == body1Updated {
			foundUpdated = true
		}
		if *c.Body == body2 {
			foundUnchanged = true
		}
	}
	g.Expect(foundUpdated).To(BeTrue())
	g.Expect(foundUnchanged).To(BeTrue())
}
