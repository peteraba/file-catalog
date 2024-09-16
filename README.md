# file-catalog

Find duplicates by partial name matches and or hashes.

## Preparation

To install either [download the latest binary](https://github.com/peteraba/cloudy-files/releases), or if you have go
installed, you can check out the code and install the application manually.

If you download the last release, make sure to move it to a directory that is in your `$PATH`.

If you checked out the code, you need to build and/or install it yourself via running `go build .` or `go install .`.

## Usage

Below we will be using `db.csv` as an example. You might want to create a central, easy to find file instead such as
`~/.config/file-catalog/db.csv` or something similar.

### Get help

Run `file-catalog` or `file-catalog --help`, perhaps `file-catalog -h`

### Build your database

Scanning directories will literally find all files inside the given directories. Once the scanning is done it will
result in the following:

1. Files already found in the database will be skipped / ignored.
2. Files which are not yet in the database will be added with their hash values.
3. Files which are not found but which can be found in the database, will be removed from the database.

`file-catalog scanDir db.csv ~/dir1 ~/dir2 ~/dir2`

*Note 1:* If a file changes that's already in the database, it will be ignored for now, even if it's size changes.

*Note 2:* The hash is a simple md5 hash calculated from the first MB of the file. If the file is longer than one MB,
then the whole file is used to calculate the md5 hash.

### Find duplicates (by hash and size or partial file names)

This command will not scan the file system, only search the database previously created.

`file-catalog duplicates db.csv`

*Note:* So one tricky thing about finding duplicates, especially when it comes to videos, is that you might keep cutting
your videos so that their sizes and hash values could be different but these files might still be duplicated. This mode
helps as it can find duplicates for you, as long as you're consistent on your naming.

Example:
- Original file: `DSC_5070-Verbessert-RR-Bearbeitet.jpg`
- Resized file: `2021.12.31-DSC_5070-Verbessert-RR-Bearbeitet-1024x768.jpg`

The files here suggest that that original was resized, meaning they will have different sizes and hashes, yet still
duplicated. But because we build a list of keywords based on `-` separator. we will have an interesting set of search
terms for each:

For the original file:
- `DSC_5070`
- `Verbessert`
- `RR`
- `Bearbeitet`

For the resized file:
- `2021.12.31`
- `DSC_5070`
- `Verbessert`
- `RR`
- `Bearbeitet`
- `1024x768`

This means that by default this would not be picked up by `file-catalog` as a duplicate, as it only finds matches with
15 matching characters or more. However if we lower the `search-min-length` option to 10, we could find these duplicates
as both `Verbessert` and `Bearbeitet` are 10 characters long and both file have them as search terms.

So to recap, to find these files as duplicates one would need to run the command something like this:

`file-catalog duplicates --search-min-length 10 db.csv`

or:

`file-catalog duplicates --search-min-length=10 db.csv`

### Find files by search term

In this mode `file-catalog` will not scan files, only use the existing database to find matches. It can take multiple
parameters and all of them will be used with `AND` logic.

Example:

`file-catalog termSearch db.csv foo bar`

This will result in `file-catalog` searching the `db.csv` file for files which have both `foo` and `bar` in their names.

*Note 1:* For now, `file-catalog` will always search in a case-agnostic manner, meaning that `Foo` `foo` and `fOo` are
considered to be the same both in file names and search terms.

### Find files by file name

This mode is similar to finding files by search name, but it first turns a file name into search terms before running
the search. This means that it can find partial matches.

Example:

`file-catalog fileSearch db.csv hello.txt`

It would find not only exact matches, but also for example `31.08.2024-hello-foo-bar-1024x768.csv`.

### Stats

This action is mostly useful for debugging purposes, but other use cases may be possible.

`file-catalog stats db.csv`
