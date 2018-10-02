#!/usr/bin/env bash

# merge OCR text files

outpdf="$1"
filesperpdf="$2"
shift 2

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

function convert_images ()
{
	echo "processing images..."

	numimages="$#"

	get_num_chunks "$numimages" "$filesperpdf"

	for ((i=1;i<="$chunks";i++)); do
		ndx="$(expr \( \( "$i" - 1 \) \* "$filesperpdf" \) + 1)"
		end="$(expr $ndx + $filesperpdf - 1)"
		[ "$end" -gt "$numimages" ] && end="$numimages"
		len="$(expr $end - $ndx + 1)"

		get_next_pdf
		pdfs+=("$pdf")

		printf "[%3d/%3d] converting %3d images (%3d-%3d) into pdf: [%s]\n" "$i" "$chunks" "$len" "$ndx" "$end" "$pdf"

		convert -density 150 "${@:$ndx:$len}" "$pdf"
	done
}

function merge_pdfs ()
{
	echo "merging ${#pdfs[@]} pdfs into pdf: [$outpdf]"

	gs -dBATCH -dNOPAUSE -q -sDEVICE=pdfwrite -sOutputFile="$outpdf" "${pdfs[@]}"
}

do_cleanup ()
{
	echo "cleaning up..."

	rm -f "${pdfs[@]}"
}

convert_images "$@"

merge_pdfs

do_cleanup

exit 0
