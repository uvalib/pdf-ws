#!/usr/bin/env bash

# merge OCR text files

# required arguments
title=""
author=""
year=""
rights=""
logofile=""
outpdf=""

# optional arguments
numimagesperpdf="50"

pdfs=()
count="0"

workdir="$(dirname "$outpdf")"
cd "$workdir" || exit 1

function get_num_chunks ()
{
	local items="$1"
	local chunksize="$2"

	chunks="$(expr \( \( "$items" - 1 \) / "$chunksize" \) + 1)"

	echo "[$items] items will be split into [$chunks] chunks of size [$chunksize]"
}

function get_next_pdf ()
{
	((count++))

	pdf="$(printf "partial-%04d.pdf" "$count")"
}

function create_cover_image ()
{
	echo "creating cover page..."

	width="1024"
	height="1320"

	# captions with margins
	capmargin="100"
	capwidth="$(expr "$width" - 2 \* "$capmargin")"

	logowidth="$(identify -format "%w" "$logofile")"
	logoinset="$(expr \( "$width" - "$logowidth" \) / 2)"

	today="$(date +%Y-%m-%d)"
	pointreg="20"
	pointbig="30"

	rights="$(echo -e "$rights" | grep -v "/catalog/" | sed -e 's/http:/https:/g' -e 's/\(.*html\)\(.*\)$/\1/g')"

	font="Arial"
	#font="TimesNewRoman"

	bkg="none"

	header="This book was made available courtesy of the UVA Library.\n\nNOTICE: This material may be protected by copyright law (Title 17, United States Code)"

	generated="Generation date: ${today}"

	citation=""
	if [ "$author" != "" ]; then
		citation="${citation}${author}"
		c="$(echo -n "$author" | tail -c 1)"
		[ "$c" != "." ] && citation="${citation}."
		citation="${citation} "
	fi
	if [ "$year" != "" ]; then
		citation="${citation}(${year}). "
	fi
	citation="${citation}\"${title}\" [PDF document]. Available from ${url}"

	libraryid="UVA Library ID Information:\n\n${rights}"

	footer="${generated}\n\n\n${citation}\n\n\n\n${libraryid}"

	yoffset="150"

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointreg" \
		-page "+${capmargin}+${yoffset}" caption:"${header}" header.miff
	ylast="$(identify -format "%h" header.miff)"
	(( yoffset += 100 + "$ylast" ))

	convert -page "+${logoinset}+${yoffset}" "$logofile" logo.miff
	ylast="$(identify -format "%h" logo.miff)"
	(( yoffset += 100 + "$ylast" ))

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointbig" \
		-page "+${capmargin}+${yoffset}" caption:"${title}" title.miff
	ylast="$(identify -format "%h" title.miff)"
	(( yoffset += "$ylast" ))

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointreg" \
		-page "+${capmargin}+${yoffset}" caption:"${author}" author.miff
	ylast="$(identify -format "%h" author.miff)"
	(( yoffset += 100 + "$ylast" ))

	convert -size "${capwidth}x" -gravity center -fill black -background "$bkg" -font "$font" -pointsize "$pointreg" \
		-page "+${capmargin}+${yoffset}" caption:"${footer}" footer.miff

	cat header.miff logo.miff title.miff author.miff footer.miff \
		| convert -size "${width}x${height}" xc:white - -flatten cover.png

	rm -f *.miff
}

function create_partial_pdfs ()
{
	echo "processing images..."

	numimages="$#"

	get_num_chunks "$numimages" "$numimagesperpdf"

	# determine a reasonable maximum height for limiting oddly-shaped images such as spines
	maxheight="$(identify "$@" 2>/dev/null | awk '
BEGIN {
	limit = 1024 * 1.5;
	maxh = 0;
}
{
	split($3, wh, "x");
	h = wh[2];
	if (h < limit && h > maxh)
		maxh = h;
	}
END {
	print maxh;
}')"

	for ((i=1;i<="$chunks";i++)); do
		ndx="$(expr \( \( "$i" - 1 \) \* "$numimagesperpdf" \) + 1)"
		end="$(expr "$ndx" + "$numimagesperpdf" - 1)"
		[ "$end" -gt "$numimages" ] && end="$numimages"
		len="$(expr "$end" - "$ndx" + 1)"

		get_next_pdf
		pdfs+=("$pdf")

		printf "[%3d/%3d] converting %3d images (%3d-%3d) into pdf: [%s]\n" "$i" "$chunks" "$len" "$ndx" "$end" "$pdf"

		convert -resize "x${maxheight}>" "${@:$ndx:$len}" "$pdf"
	done
}

function merge_partial_pdfs ()
{
	echo "merging ${#pdfs[@]} pdfs into pdf: [$outpdf]"

	gs -dBATCH -dNOPAUSE -q -sDEVICE=pdfwrite -sOutputFile="$outpdf" "${pdfs[@]}"
}

function do_cleanup ()
{
	echo "cleaning up..."

	rm -f "${pdfs[@]}"
}

### parse command line

while [ "$#" -gt "0" ]; do
	arg="$1"
	val="$2"

	case $arg in
		-a ) author="$val"; shift; shift ;;
		-l ) logofile="$val"; shift; shift ;;
		-n ) numimagesperpdf="$val"; shift; shift ;;
		-o ) outpdf="$val"; shift; shift ;;
		-r ) rights="$val"; shift; shift ;;
		-t ) title="$val"; shift; shift ;;
		-y ) year="$val"; shift; shift ;;
		-- ) shift; break ;;
		-* ) echo "unknown option: [$arg]"; exit 1 ;;
		 * ) break ;;
	esac
done

printf "%15s : [%s]\n" "title" "$title"
printf "%15s : [%s]\n" "author" "$author"
printf "%15s : [%s]\n" "year" "$year"
printf "%15s : [%s]\n" "rights" "$rights"
printf "%15s : [%s]\n" "logofile" "$logofile"
printf "%15s : [%s]\n" "outpdf" "$outpdf"
printf "%15s : [%s]\n" "numimagesperpdf" "$numimagesperpdf"
for f in "$@"; do
	printf "%15s : [%s]\n" "file" "$f"
done

create_cover_image

create_partial_pdfs "cover.png" "$@"

merge_partial_pdfs

do_cleanup

exit 0
