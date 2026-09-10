- **Devin reviews a pull request while its checks run, not after.** A new GitHub Actions
  workflow, `devin-review.yml`, asks the Devin API for a review as soon as a pull request opens
  or gains commits, draft or not, where the automatic review had waited for the draft to be
  marked ready — which here happens only once CI has cleared. It needs the repository secret
  `DEVIN_API_TOKEN`; see `CONTRIBUTING.md`.
