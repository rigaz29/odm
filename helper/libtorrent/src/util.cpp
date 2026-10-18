#include "util.hpp"

#include <cstdio>
#include <filesystem>
#include <fstream>
#include <iterator>
#include <system_error>

#ifdef _WIN32
#include <windows.h>
#include <shellapi.h>
#include <io.h>
#else
#include <unistd.h>
#endif

namespace fs = std::filesystem;

namespace odm {

std::vector<std::string> args(int argc, char* argv[]) {
  std::vector<std::string> out;
#ifdef _WIN32
  (void)argc;
  (void)argv;
  int n = 0;
  LPWSTR* wargv = CommandLineToArgvW(GetCommandLineW(), &n);
  if (wargv == nullptr) return out;
  for (int i = 1; i < n; ++i) {
    int len = WideCharToMultiByte(CP_UTF8, 0, wargv[i], -1, nullptr, 0, nullptr, nullptr);
    std::string s(len > 0 ? len : 1, '\0');
    if (len > 0) WideCharToMultiByte(CP_UTF8, 0, wargv[i], -1, s.data(), len, nullptr, nullptr);
    s.resize(len > 0 ? len - 1 : 0);
    out.push_back(std::move(s));
  }
  LocalFree(wargv);
#else
  for (int i = 1; i < argc; ++i) out.emplace_back(argv[i]);
#endif
  return out;
}

std::string native(std::string path) {
#ifdef _WIN32
  for (char& c : path) {
    if (c == '/') c = '\\';
  }
#endif
  return path;
}

std::string portable(std::string path) {
#ifdef _WIN32
  for (char& c : path) {
    if (c == '\\') c = '/';
  }
#endif
  return path;
}

bool read_file(std::string const& path, std::vector<char>& out) {
  std::ifstream in(fs::u8path(path), std::ios::binary);
  if (!in) return false;
  out.assign(std::istreambuf_iterator<char>(in), std::istreambuf_iterator<char>());
  return !in.bad();
}

bool write_file(std::string const& path, std::vector<char> const& data, std::string& err) {
  fs::path target = fs::u8path(path);
  fs::path tmp = target;
  tmp += ".tmp";
  std::error_code ec;
  if (target.has_parent_path()) fs::create_directories(target.parent_path(), ec);
#ifdef _WIN32
  FILE* f = _wfopen(tmp.c_str(), L"wb");
#else
  FILE* f = std::fopen(tmp.c_str(), "wb");
#endif
  if (f == nullptr) {
    err = "cannot create " + tmp.u8string();
    return false;
  }
  bool ok = std::fwrite(data.data(), 1, data.size(), f) == data.size() && std::fflush(f) == 0;
#ifdef _WIN32
  ok = ok && _commit(_fileno(f)) == 0;
#else
  ok = ok && fsync(fileno(f)) == 0;
#endif
  ok = std::fclose(f) == 0 && ok;
  if (ok) fs::rename(tmp, target, ec);
  if (!ok || ec) {
    err = "cannot write " + target.u8string();
    if (ec) err += ": " + ec.message();
    fs::remove(tmp, ec);
    return false;
  }
  return true;
}

void remove_file(std::string const& path) {
  std::error_code ec;
  fs::remove(fs::u8path(path), ec);
}

std::string hex(char const* data, std::size_t size) {
  static char const digits[] = "0123456789abcdef";
  std::string out;
  out.reserve(size * 2);
  for (std::size_t i = 0; i < size; ++i) {
    auto b = static_cast<unsigned char>(data[i]);
    out += digits[b >> 4];
    out += digits[b & 15];
  }
  return out;
}

std::string base64(std::vector<unsigned char> const& data) {
  static char const table[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  std::string out;
  out.reserve((data.size() + 2) / 3 * 4);
  std::size_t i = 0;
  for (; i + 2 < data.size(); i += 3) {
    unsigned n = (data[i] << 16) | (data[i + 1] << 8) | data[i + 2];
    out += table[(n >> 18) & 63];
    out += table[(n >> 12) & 63];
    out += table[(n >> 6) & 63];
    out += table[n & 63];
  }
  if (i + 1 == data.size()) {
    unsigned n = data[i] << 16;
    out += table[(n >> 18) & 63];
    out += table[(n >> 12) & 63];
    out += "==";
  } else if (i + 2 == data.size()) {
    unsigned n = (data[i] << 16) | (data[i + 1] << 8);
    out += table[(n >> 18) & 63];
    out += table[(n >> 12) & 63];
    out += table[(n >> 6) & 63];
    out += '=';
  }
  return out;
}

}  // namespace odm
