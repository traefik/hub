#!/bin/sh

diff -Nau $@  | sed -re '1,2 s/\t.*//'

exit 0
