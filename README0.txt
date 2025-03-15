
Image Finder Tool
-----------------

Usage:
  ./image-finder scan --folder=PATH [--database=PATH] [--prefix=NAME] [--force]
  ./image-finder search --image=PATH [--database=PATH] [--threshold=VALUE] [--prefix=NAME]

Examples:
  ./image-finder scan --folder=/path/to/images --prefix=ExternalDrive1
  ./image-finder search --image=/path/to/query.jpg --threshold=0.85


go build -o image-finder main.go

MORE EXAMPLES:

DATABASE="/Users/z/MBJ Dropbox/A B/ab/dev/macpro14/run/golang/match_image3/images3.db"
FOLDER="/Users/z/Downloads/_vittorio/"
PREFIX="DISK0"
IMAGE="/Users/z/Downloads/SFA_043_30.jpg"

./image-finder scan --folder=$FOLDER --database=$DATABASE --prefix=$PREFIX
./image-finder search --image=$IMAGE --database=$DATABASE