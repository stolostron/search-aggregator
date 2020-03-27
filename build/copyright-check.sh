#!/bin/bash

#IBM Confidential
#OCO Source Materials

#(C) Copyright IBM Corporation 2019 All Rights Reserved
#The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.

#LINE1="${COMMENT_PREFIX}IBM Confidential"
CHECK0="IBM Confidential"
#LINE2="${COMMENT_PREFIX}OCO Source Materials"
CHECK1="OCO Source Materials"


#LINE4="${COMMENT_PREFIX}(C) Copyright IBM Corporation 2019 All Rights Reserved"
CHECK2="(C) Copyright IBM Corporation 2019 All Rights Reserved"
CHECK2a="(C) Copyright IBM Corporation 2020 All Rights Reserved"
#LINE5="${COMMENT_PREFIX}The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office."
CHECK3="The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office."

#LIC_ARY to scan for
LIC_ARY=("$CHECK0" "$CHECK1" "$CHECK2" "$CHECK3")
LIC_ARY_SIZE=${#LIC_ARY[@]}

#Used to signal an exit
ERROR=0


echo "##### Copyright check #####"
#Loop through all files. Ignore .FILENAME types
for f in `find . -type f -iname "*.go" ! -path "./build-harness/*" ! -path "./sslcert/*" ! -path "./vendor/*"`; do
  if [ ! -f "$f" ] || [ "$f" = "./build-tools/copyright-check.sh" ]; then
    continue
  fi

  FILETYPE=$(basename ${f##*.})
  case "${FILETYPE}" in
  	sh | go)
  		COMMENT_PREFIX=""
  		;;
  	*)
      continue
  esac

  #Read the first 10 lines, most Copyright headers use the first 6 lines.
  HEADER=`head -10 $f`
  printf " Scanning $f . . . "

  #Check for all copyright lines
  for i in `seq 0 $((${LIC_ARY_SIZE}+1))`; do
    #Add a status message of OK, if all copyright lines are found
    if [ $i -eq ${LIC_ARY_SIZE} ]; then
      printf "OK\n"
    else
      if [[ $i == 2
        && "$HEADER" != *"${CHECK2}"*
        && "$HEADER" != *"${CHECK2a}"* ]]; then
        printf "Missing copyright\n  >>Could not find [${LIC_ARY[$i]}] in the file $f\n"
        ERROR=1
        break
      fi
      #Validate the copyright line being checked is present
      if [[ "$HEADER" != *"${LIC_ARY[$i]}"* && $i != 2 ]]; then
        printf "Missing copyright\n  >>Could not find [${LIC_ARY[$i]}] in the file $f\n"
        ERROR=1
        break
      fi
    fi
  done
done

echo "##### Copyright check ##### ReturnCode: ${ERROR}"
exit $ERROR