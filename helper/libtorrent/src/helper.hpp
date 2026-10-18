// Helper is odm-libtorrent's protocol mode: one libtorrent session that ODM
// drives through commands on stdin, answered with replies and events on
// stdout (PROTOCOL.md).
#pragma once

#include <libtorrent/alert.hpp>
#include <libtorrent/info_hash.hpp>
#include <libtorrent/session.hpp>
#include <libtorrent/torrent_handle.hpp>
#include <nlohmann/json.hpp>

#include <chrono>
#include <condition_variable>
#include <deque>
#include <map>
#include <memory>
#include <mutex>
#include <string>

namespace odm {

// Torrent is a torrent of the session, named by ODM's key.
struct Torrent {
  std::string key;
  lt::torrent_handle handle;
  lt::info_hash_t info_hashes;
  // resume is where its resume data is kept, "" for nowhere.
  std::string resume;
  bool has_metadata = false;
  // finishing: the torrent finished and its cache is being written out;
  // "finished" follows once it is on the disk.
  bool finishing = false;
  // saving: a save_resume_data is under way.
  bool saving = false;
  // removing: remove was asked; the torrent only waits for libtorrent to
  // let it go (and to delete its files when delete_files is set).
  bool removing = false;
  bool delete_files = false;
};

class Helper {
 public:
  // state is the file that keeps the session's DHT nodes between runs, ""
  // for none.
  explicit Helper(std::string state);
  int run();

 private:
  using json = nlohmann::json;

  void read_input();
  void handle_line(std::string const& line);
  json dispatch(std::string const& cmd, json const& msg);

  json cmd_settings(json const& msg);
  json cmd_inspect(json const& msg);
  json cmd_add(json const& msg);
  json cmd_remove(json const& msg);
  json cmd_priorities(json const& msg);
  json cmd_rename(json const& msg);
  json cmd_move(json const& msg);
  json cmd_limits(json const& msg);
  json cmd_connect(json const& msg);
  json cmd_status(json const& msg);
  json cmd_pieces(json const& msg);
  json cmd_file_progress(json const& msg);

  void handle_alerts();
  void on_alert(lt::alert* a);
  void on_metadata(Torrent& t);
  void save_resume(Torrent& t);
  void shutdown();

  Torrent& torrent(json const& msg);
  Torrent* find(lt::torrent_handle const& h);
  Torrent* find_removing(lt::info_hash_t const& ih);
  void forget(std::string const& key);
  void emit(json const& j);

  std::string state_;
  std::unique_ptr<lt::session> ses_;
  std::map<std::string, Torrent> torrents_;
  // outstanding counts the save_resume_data calls not answered yet;
  // flushing the flush_cache calls of quitting.
  int outstanding_ = 0;
  int flushing_ = 0;
  bool quitting_ = false;

  std::chrono::milliseconds status_interval_{1000};
  std::chrono::steady_clock::time_point next_status_;
  std::chrono::steady_clock::time_point next_resume_check_;

  // Shared with the thread that reads stdin and with libtorrent's alert
  // notification.
  std::mutex mu_;
  std::condition_variable wake_;
  std::deque<std::string> lines_;
  bool eof_ = false;
  bool alerts_ = false;
};

}  // namespace odm
