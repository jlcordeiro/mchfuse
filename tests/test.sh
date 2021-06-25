MNT_PATH=$(mount | grep "My Cloud" | awk -F' ' '{print $6}')

if [ ${#MNT_PATH} -eq 0 ]; then
    echo "Not mounted. Closing." ;
    exit
else echo "Mounted."
fi

## ---------------------------

printf "Touching file... "

DST_PATH=${MNT_PATH}/fcp
rm -f ${DST_PATH}
touch ${DST_PATH}
touch ${DST_PATH}
rm ${DST_PATH}
echo "OK"

## ---------------------------

printf "Copying new file... "

DST_PATH=${MNT_PATH}/fcp
rm -f ${DST_PATH}
cp f ${DST_PATH}

if echo "098ebf15fde7dd3b2cf667c0bba8657817bcbf07  "${DST_PATH} | sha1sum --quiet --check; then
    echo "OK"
else
    echo "FAIL Copied file is corrupted." ;
    exit
fi

printf "Touching pre-existing file... "

touch ${DST_PATH}

if echo "098ebf15fde7dd3b2cf667c0bba8657817bcbf07  "${DST_PATH} | sha1sum --quiet --check; then
    echo "OK"
else
    echo "FAIL Copied file is corrupted." ;
    exit
fi

rm -f ${DST_PATH}

## ---------------------------

printf "Rsyncing new file... "

DST_PATH=${MNT_PATH}/frsync
rm -f ${DST_PATH}
cp f ${DST_PATH}

if echo "098ebf15fde7dd3b2cf667c0bba8657817bcbf07  "${DST_PATH} | sha1sum --quiet --check; then
    echo "OK"
else
    echo "FAIL rsync result file is corrupted." ;
    exit
fi

rm -f ${DST_PATH}

## ---------------------------

printf "Appending to file... "

DST_PATH=${MNT_PATH}/fcp
rm -f ${DST_PATH}
cp f ${DST_PATH}
echo "asd" >> ${DST_PATH}

if echo "aca40e79bdecdb1d62247a59c278abd45945c508 "${DST_PATH} | sha1sum --quiet --check; then
    echo "OK"
else
    echo "FAIL File is corrupted after cat." ;
    exit
fi

rm ${DST_PATH}

## ---------------------------

printf "Appending to non-existing file... "

DST_PATH=${MNT_PATH}/catd
rm -f ${DST_PATH}

echo "test string 1." >> ${DST_PATH}
if echo "25e273880beec0bb40e751bdf36753f5af729bdb "${DST_PATH} | sha1sum --quiet --check; then
    printf " ... "
else
    echo "FAIL File is corrupted after first cat." ;
    exit
fi

echo "another test string" >> ${DST_PATH}
if echo "382859c0d306e528df3a792188a3134f8c069bb6 "${DST_PATH} | sha1sum --quiet --check; then
    echo "OK"
else
    echo "FAIL File is corrupted after second cat." ;
    exit
fi

## ---------------------------

printf "Appending to non-existing file v2... "

DST_PATH=${MNT_PATH}/catd

echo "test string 1." > ${DST_PATH}
if echo "25e273880beec0bb40e751bdf36753f5af729bdb "${DST_PATH} | sha1sum --quiet --check; then
    printf " ... "
else
    echo "FAIL File is corrupted after first cat." ;
    exit
fi

echo "another test string" >> ${DST_PATH}
if echo "382859c0d306e528df3a792188a3134f8c069bb6 "${DST_PATH} | sha1sum --quiet --check; then
    echo "OK"
else
    echo "FAIL File is corrupted after second cat." ;
    exit
fi

rm -f ${DST_PATH}
