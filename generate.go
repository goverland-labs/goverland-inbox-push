//go:generate mockgen -destination=internal/sender/mocks_test.go -package=sender github.com/goverland-labs/inbox-push/internal/sender UsersFinder,SettingsProvider,CoreDataProvider,DataManipulator,MessageSender,PushManipulator

package main