// Small helpers of odm-libtorrent: the command line, files and encodings.
#pragma once

#include <cstddef>
#include <string>
#include <vector>

namespace odm {

// args returns the program's arguments, without its name, in UTF-8. On
// Windows main's argv is in the ANSI code page, which cannot hold every path.
std::vector<std::string> args(int argc, char* argv[]);

// native turns a path ODM sent (UTF-8, with /) into the system's separators;
// portable turns one libtorrent gave back into /.
std::string native(std::string path);
std::string portable(std::string path);

// read_file reads the whole file at path into out.
bool read_file(std::string const& path, std::vector<char>& out);

// write_file writes data to path through a temporary file that is flushed to
// the disk before it replaces path, so a power cut leaves either the old file
// or the new one. err says what failed.
bool write_file(std::string const& path, std::vector<char> const& data, std::string& err);

// remove_file removes the file at path, if there is one.
void remove_file(std::string const& path);

std::string hex(char const* data, std::size_t size);
std::string base64(std::vector<unsigned char> const& data);

}  // namespace odm
