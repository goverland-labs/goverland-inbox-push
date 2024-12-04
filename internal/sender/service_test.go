package sender

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/goverland-labs/goverland-inbox-api-protocol/protobuf/inboxapi"
	"github.com/stretchr/testify/require"
)

func TestGetToken(t *testing.T) {
	for name, tc := range map[string]struct {
		sp      func(ctrl *gomock.Controller) SettingsProvider
		token   string
		wantErr bool
	}{
		"correct token": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				m := NewMockSettingsProvider(ctrl)
				m.EXPECT().
					GetPushToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&inboxapi.PushTokenResponse{
						Token: "token",
					}, nil)

				return m
			},
			token:   "token",
			wantErr: false,
		},
		"on error": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				m := NewMockSettingsProvider(ctrl)
				m.EXPECT().
					GetPushToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("internal error"))

				return m
			},
			token:   "token",
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := &Service{
				settings: tc.sp(ctrl),
			}

			actual, err := service.GetToken(context.Background(), uuid.New())
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.token, actual)
		})
	}
}

func TestGetTokens(t *testing.T) {
	for name, tc := range map[string]struct {
		sp      func(ctrl *gomock.Controller) SettingsProvider
		list    []TokenDetails
		wantErr bool
	}{
		"correct list": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				m := NewMockSettingsProvider(ctrl)
				m.EXPECT().
					GetPushTokenList(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&inboxapi.PushTokenListResponse{
						Tokens: []*inboxapi.PushTokenDetails{
							{
								Token:      "token_1",
								DeviceUuid: "device_1",
							},
							{
								Token:      "token_2",
								DeviceUuid: "device_2",
							},
						},
					}, nil)

				return m
			},
			list: []TokenDetails{
				{
					Token:      "token_1",
					DeviceUUID: "device_1",
				},
				{
					Token:      "token_2",
					DeviceUUID: "device_2",
				},
			},
			wantErr: false,
		},
		"on error": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				m := NewMockSettingsProvider(ctrl)
				m.EXPECT().
					GetPushTokenList(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("internal error"))

				return m
			},
			list:    nil,
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := &Service{
				settings: tc.sp(ctrl),
			}

			actual, err := service.GetTokens(context.Background(), uuid.New())
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.list, actual)
		})
	}
}

func TestRequestHashGeneration(t *testing.T) {
	req := request{
		body:       "body",
		title:      "title",
		imageURL:   "image",
		userID:     uuid.MustParse("3fca2745-4adc-4fdb-b56c-ef07e38aea6a"),
		deviceUUID: "uuid_1",
		proposals:  []string{"str1", "str2", "str3"},
		template:   1,
	}

	expected := "14e3642c4703478f89c1380ae92b2a3f"

	t.Run("hash generation", func(t *testing.T) {
		actual := req.hash()
		require.Equal(t, expected, actual)
	})

	t.Run("twice generation", func(t *testing.T) {
		require.Equal(t, req.hash(), req.hash())
	})
}
