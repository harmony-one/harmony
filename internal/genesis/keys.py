import argparse

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Create Foundational Keys List from Internal Record."
    )
    parser.add_argument(
        "-sheet",
        default="allkeys-sheet.txt",
        dest="sheet",
        help="tab seperate ecdsa and bls keys",
        type=str,
    )
    parser.add_argument(
        "-foundational",
        default="allkeys-foundational-go.txt",
        dest="foundational",
        help="file compatible with foundational go",
        type=str,
    )
    parser.add_argument(
        "-index",
        default=0,
        dest="index",
        help="index of where you want to start from",
        type=int,
    )
    args = parser.parse_args()
    g = open(args.sheet, "r")
    f = open(args.foundational, "w")
    index = args.index
    for myline in g:
        ecdsa, bls = myline.strip().split("\t")
        string = (
            "{Index:"
            + '"'
            + str(index)
            + '"'
            + ","
            + " "
            + "Address:"
            + '"'
            + ecdsa
            + '"'
            + ","
            + " "
            + "BLSPublicKey:"
            + '"'
            + bls
            + '"'
            + "}"
            + ","
        )
        f.write(string + "\n")
        index = index + 1
    g.close()
