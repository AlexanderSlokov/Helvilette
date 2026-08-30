
up:
	vagrant up
	@echo ""
	@echo "================================================================"
	@echo "Vagrant environment is up! Othela, Agent, and Gitea are running."
	@echo "Next steps for First Light manual test:"
	@echo "1. Access Gitea at http://192.168.121.10:3000"
	@echo "2. Register a new user named 'helvi-test' (the first user becomes admin)"
	@echo "3. Create a new repository named 'baseline'"
	@echo "4. Push your test playbook to this repository on the 'main' branch."
	@echo "5. Watch Othela logs: make othela -> sudo journalctl -u othela -f"
	@echo "================================================================"

setup-repo:
	cd vagrant/baseline-repo && \
	git init && \
	git add . && \
	git commit -m "Initial commit for First Light test" && \
	git branch -M main && \
	git remote add origin http://192.168.121.10:3000/helvi-test/baseline.git && \
	git push -u origin main


reload:
	vagrant reload --provision

othela:
	vagrant ssh othela

agent:
	vagrant ssh agent1