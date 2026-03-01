package mailer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	appconfig "github.com/user/gitlens-pro/internal/config"
)

type SESSender struct {
	client *ses.Client
}

func NewSESSender(cfg appconfig.SESConfig) (*SESSender, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &SESSender{
		client: ses.NewFromConfig(awsCfg),
	}, nil
}

func (s *SESSender) Send(ctx context.Context, msg *Message) error {
	input := &ses.SendEmailInput{
		Source: aws.String(msg.From),
		Destination: &types.Destination{
			ToAddresses: []string{msg.To},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(msg.Subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(msg.HTML),
					Charset: aws.String("UTF-8"),
				},
			},
		},
	}

	if msg.Text != "" {
		input.Message.Body.Text = &types.Content{
			Data:    aws.String(msg.Text),
			Charset: aws.String("UTF-8"),
		}
	}

	_, err := s.client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("SES send: %w", err)
	}

	return nil
}
