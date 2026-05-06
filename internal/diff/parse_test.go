package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleModified = `diff --git a/foo.yml b/foo.yml
index 1234567..89abcde 100644
--- a/foo.yml
+++ b/foo.yml
@@ -1,3 +1,4 @@
 a
-b
+B
+c
 d
`

const sampleAdded = `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+hello
+world
`

const sampleDeleted = `diff --git a/old.txt b/old.txt
deleted file mode 100644
index abc1234..0000000
--- a/old.txt
+++ /dev/null
@@ -1,1 +0,0 @@
-bye
`

const sampleRenamed = `diff --git a/from.txt b/to.txt
similarity index 95%
rename from from.txt
rename to to.txt
index abc..def 100644
--- a/from.txt
+++ b/to.txt
@@ -1,2 +1,2 @@
-old
+new
 same
`

func TestParseModified(t *testing.T) {
	fs, err := Parse([]byte(sampleModified))
	require.NoError(t, err)
	require.Len(t, fs, 1)
	require.Equal(t, "foo.yml", fs[0].Path)
	require.Equal(t, "modified", fs[0].Status)
	require.Len(t, fs[0].Hunks, 1)
	require.Equal(t, 1, fs[0].Hunks[0].NewStart)
	require.Equal(t, 4, fs[0].Hunks[0].NewLines)
	require.Len(t, fs[0].Hunks[0].Lines, 5)
}

func TestParseAdded(t *testing.T) {
	fs, err := Parse([]byte(sampleAdded))
	require.NoError(t, err)
	require.Len(t, fs, 1)
	require.Equal(t, "added", fs[0].Status)
	require.Equal(t, "new.txt", fs[0].Path)
}

func TestParseDeleted(t *testing.T) {
	fs, err := Parse([]byte(sampleDeleted))
	require.NoError(t, err)
	require.Len(t, fs, 1)
	require.Equal(t, "deleted", fs[0].Status)
}

func TestParseRenamed(t *testing.T) {
	fs, err := Parse([]byte(sampleRenamed))
	require.NoError(t, err)
	require.Len(t, fs, 1)
	require.Equal(t, "renamed", fs[0].Status)
	require.Equal(t, "from.txt", fs[0].OldPath)
	require.Equal(t, "to.txt", fs[0].Path)
}

func TestParseMultiFile(t *testing.T) {
	combined := sampleModified + sampleAdded
	fs, err := Parse([]byte(combined))
	require.NoError(t, err)
	require.Len(t, fs, 2)
	require.Equal(t, "modified", fs[0].Status)
	require.Equal(t, "added", fs[1].Status)
}

func TestParseEmpty(t *testing.T) {
	fs, err := Parse(nil)
	require.NoError(t, err)
	require.Empty(t, fs)
}
