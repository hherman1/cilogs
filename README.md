
# cilogs
Downloads every artifact from a given circle.ci job

## Install

```
go install github.com/hherman1/cilogs@latest
```

Or download the binaries [here](https://github.com/hherman1/cilogs/releases/latest)

# Usage

Run `cilogs -a <API_KEY> <JOB_URL>` and cilogs will download every artifact from the job you specified into the current directory.

The `-a` flag may be omitted by writing your API key to the file `~/.config/secrets/circleci`.

To change the output directory, use `-d <DIRECTORY>`.

To only print the artifact URLs and not actually download them, use the `-p` flag.

You can fetch a circle ci API key from your user profile.
