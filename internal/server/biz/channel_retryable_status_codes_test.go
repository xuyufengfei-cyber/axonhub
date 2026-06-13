package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestNormalizeRetryableStatusCodes(t *testing.T) {
	t.Run("sorts and deduplicates error status codes", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableStatusCodes: []int{403, 400, 403, 500},
		}

		err := NormalizeRetryableStatusCodes(settings)

		require.NoError(t, err)
		require.Equal(t, []int{400, 403, 500}, settings.RetryableStatusCodes)
	})

	t.Run("allows empty settings", func(t *testing.T) {
		require.NoError(t, NormalizeRetryableStatusCodes(nil))
		require.NoError(t, NormalizeRetryableStatusCodes(&objects.ChannelSettings{}))
	})

	t.Run("rejects non error status codes", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableStatusCodes: []int{200},
		}

		err := NormalizeRetryableStatusCodes(settings)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid retryable status code 200")
	})
}

func TestNormalizeRetryableErrorPatterns(t *testing.T) {
	t.Run("trims and deduplicates retryable error patterns", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableErrorPatterns: []objects.RetryableErrorPattern{
				{Pattern: " Console API returned 403 "},
				{Pattern: "Console API returned 403"},
				{Pattern: `Console API returned \d+`, Regex: true},
			},
		}

		err := NormalizeRetryableErrorPatterns(settings)

		require.NoError(t, err)
		require.Len(t, settings.RetryableErrorPatterns, 2)
		require.Equal(t, "Console API returned 403", settings.RetryableErrorPatterns[0].Pattern)
		require.False(t, settings.RetryableErrorPatterns[0].Regex)
		require.Nil(t, settings.RetryableErrorPatterns[0].CompiledRegex)
		require.Equal(t, `Console API returned \d+`, settings.RetryableErrorPatterns[1].Pattern)
		require.True(t, settings.RetryableErrorPatterns[1].Regex)
		require.NotNil(t, settings.RetryableErrorPatterns[1].CompiledRegex)
		require.True(t, settings.RetryableErrorPatterns[1].CompiledRegex.MatchString("Console API returned 502"))
	})

	t.Run("allows empty settings", func(t *testing.T) {
		require.NoError(t, NormalizeRetryableErrorPatterns(nil))
		require.NoError(t, NormalizeRetryableErrorPatterns(&objects.ChannelSettings{}))
	})

	t.Run("rejects invalid regex patterns", func(t *testing.T) {
		settings := &objects.ChannelSettings{
			RetryableErrorPatterns: []objects.RetryableErrorPattern{
				{Pattern: "Console API returned [", Regex: true},
			},
		}

		err := NormalizeRetryableErrorPatterns(settings)

		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid retryable error regex")
	})

	t.Run("compiles regex patterns when building runtime channel", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := authz.WithTestBypass(t.Context())
		entChannel := client.Channel.Create().
			SetName("Retryable Error Regex Channel").
			SetType(channel.TypeOpenaiFake).
			SetCredentials(objects.ChannelCredentials{}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetSettings(&objects.ChannelSettings{
				RetryableErrorPatterns: []objects.RetryableErrorPattern{
					{Pattern: "Console API returned 403"},
					{Pattern: `Console API returned \d+`, Regex: true},
				},
			}).
			SaveX(ctx)

		channelSvc := NewChannelServiceForTest(client)
		built, err := channelSvc.buildChannelWithOutbounds(entChannel)

		require.NoError(t, err)
		require.NotNil(t, built.Settings.RetryableErrorPatterns[1].CompiledRegex)
		require.True(t, built.Settings.RetryableErrorPatterns[1].CompiledRegex.MatchString("Console API returned 502"))
	})
}
