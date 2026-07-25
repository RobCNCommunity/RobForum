package app

import "testing"

func TestPostModerationTextIncludesEveryTag(t *testing.T) {
	got := postModerationText("更新公告", "正文内容", []string{"Roblox", " 攻略 ", ""})
	want := "标题：更新公告\n正文：正文内容\n标签：Roblox\n标签：攻略"
	if got != want {
		t.Fatalf("post moderation text = %q, want %q", got, want)
	}
}
