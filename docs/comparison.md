# gmt-mail compared

| | gmt-mail | Python mail-merge script | Thunderbird mail merge | Mailchimp & co. |
|---|---|---|---|---|
| Install | one binary | Python + code you maintain | Thunderbird + add-on | web account |
| Scriptable / CI | yes | yes | no | via API |
| Checks before sending | `-validate`, `-dry-run` | what you write | preview | preview |
| Sent from | your SMTP account | your SMTP account | your mail account | their servers |
| Your list stays with you | yes | yes | yes | no |

`gmt-mail` is meant for tens to a few hundred personal messages. For large newsletters that need unsubscribe handling and bounce tracking, use an email service provider.
