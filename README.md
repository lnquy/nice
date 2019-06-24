# nice
Format JSON log to human-readable log

# Usage
```shell
$ nice -h
  -f string
        Output format. Fields can be access by dot notation path, separated by comma (,)
  -files string
        List of path input log files, separated by comma (,)

Examples:
  $ nice --files 20190624.log -f time,msg
  $ myapp | nice -f time,level,msg
  $ myapp | nice --files 20190624.log,anotherlogfile.log -f time,level,msg
```

# Build from source
Require Go >= 1.11 as I'm using go module as dependencies management.
```shell
$ go get -u -v github.com/lnquy/nice
```

Test example with log from both `stdin` and file.
```shell
# Build logstdin binary file
$ cd logstdin
$ go build -o logstdin.bin .
$ mv logstdin.bin ..
$ cd ..

# Build nice
$ go mod vendor
$ go build -o nice.bin .
$ ./logstdin.bin | ./nice.bin --files 20190624.log -f time,level,msg
```

# License
This project is under the MIT License. See the [LICENSE](https://github.com/lnquy/nice/blob/master/LICENSE) file for the full license text.
