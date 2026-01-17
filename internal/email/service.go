package email

import (
	"fmt"
	"net/smtp"
)

// Service handles email sending via SMTP
type Service struct {
	host string
	port string
	from string
}

// NewService creates a new email service
func NewService(host, port, from string) *Service {
	return &Service{
		host: host,
		port: port,
		from: from,
	}
}

// SendOrderConfirmation sends an order confirmation email
func (s *Service) SendOrderConfirmation(to, orderID string, total int, items []OrderItem) error {
	shortID := orderID
	if len(orderID) > 8 {
		shortID = orderID[:8]
	}
	subject := fmt.Sprintf("【注文確認】ご注文ありがとうございます（注文番号: %s）", shortID)
	body := BuildOrderConfirmationBody(orderID, total, items)
	return s.send(to, subject, body)
}

func (s *Service) send(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, body)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	return smtp.SendMail(addr, nil, s.from, []string{to}, []byte(msg))
}
