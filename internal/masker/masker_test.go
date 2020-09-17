package masker_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/alecthomas/assert"
	"github.com/reconquest/snake-runner/internal/env"
	"github.com/reconquest/snake-runner/internal/masker"
)

const OPENSSH_PRIVATE_KEY = `
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAYEAlcr/jEutQk5Z2KlmYMQ641O8YBRKjQj0P9Kx0m2XzdRKrCjmnFHS
ruCVQg0cru6MfMn0vr8BLltCJSgZacBkZ8EjiSf4V3CYYQ4ht7t1+37IgvIoGtjq+65cdu
bPCZAbI/V7kTQ5J+2J+n4VIDNtQeC2lc4CNxZKtJ0WObUhZNpamQ70DBdLlY0EaG+7qV/Z
dIA2Af33iis0X9M/Q/3MrLStJqrqkW8dprawfMd/Cvsr08hzgmxyotAK/GE5XmDn3KKHf0
VuM9WCx4lDF1T8D2318hU2weQ7BMISLO/I9aS2Y2gGDRRK6In285mUh619Qd6L2STZBHYn
o24B4G7CTThJoD4W6JYPoZzxANvCMP5rGgF3fBBWAv3dkgQtFRrrIXmirXZPfczutZVTIC
6w6tgEaUY+N1hzr0lvEzPngfhDOxbRexOnZr13S/FxmVznY+UV7RMpigrbDKkPpIgfLj05
MGD+nWsDr3dZ0ObvuoMKu65Z1MmlsOaKAzAOoO1VAAAFiFfwgpRX8IKUAAAAB3NzaC1yc2
EAAAGBAJXK/4xLrUJOWdipZmDEOuNTvGAUSo0I9D/SsdJtl83USqwo5pxR0q7glUINHK7u
jHzJ9L6/AS5bQiUoGWnAZGfBI4kn+FdwmGEOIbe7dft+yILyKBrY6vuuXHbmzwmQGyP1e5
E0OSftifp+FSAzbUHgtpXOAjcWSrSdFjm1IWTaWpkO9AwXS5WNBGhvu6lf2XSANgH994or
NF/TP0P9zKy0rSaq6pFvHaa2sHzHfwr7K9PIc4JscqLQCvxhOV5g59yih39FbjPVgseJQx
dU/A9t9fIVNsHkOwTCEizvyPWktmNoBg0USuiJ9vOZlIetfUHei9kk2QR2J6NuAeBuwk04
SaA+FuiWD6Gc8QDbwjD+axoBd3wQVgL93ZIELRUa6yF5oq12T33M7rWVUyAusOrYBGlGPj
dYc69JbxMz54H4QzsW0XsTp2a9d0vxcZlc52PlFe0TKYoK2wypD6SIHy49OTBg/p1rA693
WdDm77qDCruuWdTJpbDmigMwDqDtVQAAAAMBAAEAAAGAKr29pT6CtYS9xkCv4bScSPk/qv
qfOxbu+rcX3j8/LZABrpYNF1WRlCyo6ivrn06Z75GAsFy37Y5ElW2XssEe5SwLA7vP9MM/
95GctVxcEeLfO93065QxmJbr81Fwif4AgIYvOAN6u5Xf5QXM6V9DMaT49E3P+//0WQzppp
W0bZ6Qc1P89uY+vvF57ILVPUMIPWHIB1F8LISfcShJyRDGMhFrxrjGLbFS+JrwSOpzqaJ5
ovhzXSesniBWwiTLxc5Iar5r8C1FDmZfkdY2xj6Jw4w5Ca7n00JUnBlBYXFFUEy7YmhvZk
C4H+WYua57Duih+/ofLPlx+oHGV34BJZsniheMHwP1XItbRaOavzUazohSXwPFnHQbk7bu
L4DN4MkNfD/W/4qRyORCM0YKv84A4mSP2zgKn6BL7brgn1wzPv9O1VazV0diB/HibSVjwU
dqF6zssN2F6X+IsMI/kidJqPnDycU+HJTuEVH+3cJTypmrBVMBB73OJkk1uIv1bDkBAAAA
wQC4kI7hXTwT2Rvb9NQxFmj7tkKyEdCJlSbyhK/sYVJa+7H1GXVkNu6lnr6x5DroYxqT7r
NRa3ip+c3nDfHZCqeW7xtk7wxG6QD4Ewx5wz5XEkOSHuyg8lomXm2iIeVY6vnLNTL871SM
tfzeZ/ldVKzWjmQK0VNcSPV4aHK3QgfgUphZJmVD8OXEFFtiiTJtIZ7m7JvB2QVPfUzb1J
XiJApvrvgMRHsI4PMephjVniXtVDQ851LX90C1cBlvbDitr2AAAADBAMWgr0Z2tTVW5wzT
nAot7iD+GT6RmT4s5BoIb2VUYiK5JgyVPfRMvqzrByoGnKmOTPIw0dCmUuagU43ojZjIeK
j8xZZf/v/5qRLWhj8iHzGGdgIr+Calmhco0rh4I2dZT5xDZ+tFdJGbZ7Fzm/L6bjSVejW1
qeYX6sIgTg3zuUHfXM+71HUwvDxl1H6JZijIEOkkcXzx/3U0FmX8UHwPMDEIDsSVSNp1G1
2Jw5Tzc3EJQNZ3O0oEBfUlYLRt+Wc6sQAAAMEAwgldXHB3A3xIDd1YPxM7PlvB5da4Jxat
Z0bzEyN+5YKmMy1wOdo+MT9thlcpL9O31RIqvYUqAw+yjdKn+mFGcswa3ZDuqv8wYxestA
mOy/OzgB8H2Hy/3ceLqk2z52C1BTiYL74rwTvxsZsk45jmG8OTrYPnnGWO7BTyAaAVWIos
EH0YGg4pG4DyYYhzLBa8USYbPcZ/5xhXBNIWjGyBd35CTXHoAXRBfgIsf0nt5mVlXvjvaV
R3Gcj96ghd8X3lAAAADm9wZXJhdG9yQHZlbnVzAQIDBA==
-----END OPENSSH PRIVATE KEY-----
`

type writerCloser struct {
	*bytes.Buffer
}

func (writerCloser) Close() error {
	return nil
}

func TestMasker_Mask_ReplacesMultiLineStrings(t *testing.T) {
	test := assert.New(t)
	vars := map[string]string{
		"Y":   "yyyyy",
		"X":   "xxxxx",
		"KEY": OPENSSH_PRIVATE_KEY,
	}
	buffer := bytes.NewBuffer(nil)
	masker := masker.NewWriter(env.NewEnv(vars), []string{"X", "KEY"}, writerCloser{buffer})

	expected := `
*****
yyyyy *****

***********************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************************************
**********************************************
*********************************

test
`

	input := fmt.Sprintf(
		"\n%s\n%s %s\n%s\ntest\n",
		vars["X"], vars["Y"], vars["X"], vars["KEY"],
	)

	_, err := masker.Write([]byte(input))
	test.NoError(err)
	test.EqualValues(expected, buffer.String())

	test.EqualValues(expected, masker.Mask(input))
}

func TestMasker_Mask_ReplacesMultiLineStringsInCorrectOrder(t *testing.T) {
	test := assert.New(t)
	vars := map[string]string{
		"X": "q\nqwerty",
		"Y": "\nwqwerty\nq\nqwerty",
	}
	buffer := bytes.NewBuffer(nil)
	masker := masker.NewWriter(env.NewEnv(vars), []string{"X", "Y"}, writerCloser{buffer})

	expected := "*\n****** @ \n*******\n*\n******"

	input := vars["X"] + " @ " + vars["Y"]

	_, err := masker.Write([]byte(input))
	test.NoError(err)
	test.EqualValues(expected, buffer.String())

	test.EqualValues(expected, masker.Mask(input))
}
