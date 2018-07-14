import json
import sys
import os
import argparse

def formatFloat(v):
    return "%.2f" % v

def formatPercent(v):
    return formatFloat(v) + "%"

def formatMem(v):
    return formatFloat(float(v) / 10**6) + "MB"

class Profiler:
    def __init__(self):
        self.tps = 0
        self.tps_max = 0
        self.tps_min = sys.maxsize
        self.tps_count = 0

        self.cpu_percent = 0
        self.cpu_usr = 0
        self.cpu_sys = 0
        self.cpu_count = 0

        self.mem_rss = 0
        self.mem_rss_max = 0
        self.mem_count = 0

    def handleTPS(self, obj):
        tps = obj["TPS"]
        self.tps += tps
        self.tps_max = max(self.tps_max, tps)
        self.tps_min = min(self.tps_min, tps)
        self.tps_count += 1

    def handleCPU(self, obj):
        # http://psutil.readthedocs.io/en/latest/#psutil.Process.cpu_times
        # https://stackoverflow.com/questions/556405/what-do-real-user-and-sys-mean-in-the-output-of-time1
        # http://psutil.readthedocs.io/en/latest/#psutil.Process.cpu_percent
        self.cpu_percent += obj["percent"]
        times = json.loads(obj["times"])
        self.cpu_usr = times["user"]
        self.cpu_sys = times["system"]
        self.cpu_count += 1
    
    def handleMem(self, obj):
        # http://psutil.readthedocs.io/en/latest/#psutil.Process.memory_info
        info = json.loads(obj["info"])
        rss = info["rss"]
        self.mem_rss += rss
        self.mem_rss_max = max(self.mem_rss_max, rss)
        self.mem_count += 1

    def report(self):
        print("TPS",
            "Avg", formatFloat(self.tps / self.tps_count),
            "Min", formatFloat(self.tps_min),
            "Max", formatFloat(self.tps_max))
        print("CPU",
            "Percent (Avg)", formatPercent(self.cpu_percent / self.cpu_count),
            "Time (Usr)", str(self.cpu_usr) + "s",
            "Time (Sys)", str(self.cpu_sys) + "s")
        print("Mem",
            "RSS (Max)", formatMem(self.mem_rss_max),
            "RSS (Avg)", formatMem(self.mem_rss / self.mem_count))

def profileFile(path):
    print(path)
    profiler = Profiler()
    with open(path) as f:
        for line in f:
            obj = json.loads(line)
            if obj["lvl"] != "info":
                continue
            if obj["msg"] == "TPS Report":
                profiler.handleTPS(obj)
            elif obj["msg"] == "CPU Report":
                profiler.handleCPU(obj)
            elif obj["msg"] == "Mem Report":
                profiler.handleMem(obj)
    profiler.report()

# Example: python report_extractor.py --folder ../tmp_log/log-20180713-205431
if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="This script extracts reports from log files")
    parser.add_argument("--folder", type=str, dest="folder",
                        default="",
                        help="the path to the log folder")
    args = parser.parse_args()

    for filename in os.listdir(args.folder):
        if "leader" in filename: 
            profileFile(os.path.join(args.folder, filename))