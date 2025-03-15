
Image Finder Tool
-----------------

Usage:
  ./image-finder scan --folder=PATH [--database=PATH] [--prefix=NAME] [--force]
  ./image-finder search --image=PATH [--database=PATH] [--threshold=VALUE] [--prefix=NAME]

Examples:
  ./image-finder scan --folder=/path/to/images --prefix=ExternalDrive1
  ./image-finder search --image=/path/to/query.jpg --threshold=0.85

MORE EXAMPLES:

DATABASE="/Users/z/MBJ Dropbox/A B/ab/dev/macpro14/run/golang/match_image3/images2.db"
FOLDER="/Users/z/Downloads/_vittorio/output"
PREFIX="DISK0"
IMAGE="/Users/z/Downloads/SFA_043_30.jpg"

go run ./main.go scan --folder=$FOLDER --database=$DATABASE --prefix=$PREFIX
go run ./main.go search --image=$IMAGE --database=$DATABASE --prefix=$PREFIX

