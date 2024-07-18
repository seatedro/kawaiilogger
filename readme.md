# kawaiilogger 🔑🕵️

## this is a stupid metrics self reporter. written in go.

### feature list

    - keypress count
    - mouse click count
    - mouse distance traveled
    - scroll step count


### configuration
if you want to run fully local with no setup, just build and run as noted below.

if however, you want to store these stupid metrics in a db somewhere, do the following:

```yaml
# create a config.yaml
# ~/.config/kawaiilogger/config.yaml
database:
    type: postgres/sqlite # choose one
    url: # enter your db url if you have any
    filepath: # path to your .sqlite file if any
```

now just build and run !!

### build

```bash
env GO111MODULE=on go build
```

### run

```bash
./kawaiilogger &
# check the code for other OS logDir
bat ~/.config/kawaiilogger/kawaiilogger.log
```
