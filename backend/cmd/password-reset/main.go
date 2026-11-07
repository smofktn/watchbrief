package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"github.com/smofktn/watchbrief/backend/internal/security/password"
	"github.com/smofktn/watchbrief/backend/internal/transport/admincli"
	"github.com/smofktn/watchbrief/backend/internal/usecase"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("password-reset", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	loginID := flags.String("login-id", "", "login ID")
	secretPath := flags.String("db-password-file", "../.local/watchbrief-password-resetter-password", "local password resetter DB password file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("引数を確認してください。パスワードは引数に指定できません")
	}
	if _, err := domain.NormalizeLoginID(*loginID); err != nil {
		return errors.New("ログインIDは英数字・_・-の1〜64文字で指定してください")
	}
	first, second, err := admincli.ReadPasswords(ctx)
	if err != nil {
		return errors.New("非表示入力を完了できませんでした。端末から実行してください")
	}
	defer clear(first)
	defer clear(second)
	operationCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	uc := usecase.ResetPassword{Store: postgres.NewLocalPasswordResetter(*secretPath), Hasher: password.NewHasher()}
	if err := uc.Execute(operationCtx, *loginID, string(first), string(second)); err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentialsInput):
			return errors.New("パスワードはNFCでそろえた後の15〜128文字で入力してください")
		case errors.Is(err, usecase.ErrConfirmation):
			return errors.New("パスワードが一致しません。再設定していません")
		case errors.Is(err, domain.ErrUserNotFound):
			return errors.New("対象のログインIDは存在しません。再設定していません")
		case errors.Is(err, usecase.ErrResetPassword):
			return errors.New("再設定の準備に失敗しました。再設定していません")
		default:
			return errors.New("再設定の結果を確認できません。再実行前にDBの状態を確認してください")
		}
	}
	fmt.Fprintln(os.Stdout, "パスワードを再設定し、対象Userの既存セッションをすべて無効化しました。")
	return nil
}
