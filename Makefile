PREFIX ?= /usr/local

install:
	install -d $(PREFIX)/bin
	install -m 755 bin/kall $(PREFIX)/bin/kall
	install -d $(PREFIX)/share/man/man1
	install -m 644 man/kall.1 $(PREFIX)/share/man/man1/kall.1
	install -d $(PREFIX)/share/bash-completion/completions
	install -m 644 completions/kall.bash $(PREFIX)/share/bash-completion/completions/kall
	install -d $(PREFIX)/share/zsh/site-functions
	install -m 644 completions/_kall $(PREFIX)/share/zsh/site-functions/_kall

uninstall:
	rm -f $(PREFIX)/bin/kall
	rm -f $(PREFIX)/share/man/man1/kall.1
	rm -f $(PREFIX)/share/bash-completion/completions/kall
	rm -f $(PREFIX)/share/zsh/site-functions/_kall

.PHONY: install uninstall
