using System.Text.Json;
using System.Text.Json.Serialization;
using Realms;

namespace LazerBridge;

internal static class Program
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        WriteIndented = false,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull
    };

    public static int Main(string[] args)
    {
        if (args.Length == 0)
        {
            Console.Error.WriteLine("Usage: lazer-bridge <resolve-beatmap|list-replays|list-skins|resolve-skin> [options]");
            return 1;
        }

        string command = args[0].ToLowerInvariant();
        string? md5 = null;
        string? hash = null;
        string? name = null;
        int limit = 20;
        string? lazerPath = null;

        for (int i = 1; i < args.Length; i++)
        {
            switch (args[i])
            {
                case "--md5" when i + 1 < args.Length:
                    md5 = args[++i].Trim();
                    break;
                case "--hash" when i + 1 < args.Length:
                    hash = args[++i].Trim();
                    break;
                case "--name" when i + 1 < args.Length:
                    name = args[++i].Trim();
                    break;
                case "--limit" when i + 1 < args.Length && int.TryParse(args[++i], out int parsedLimit):
                    limit = Math.Max(1, parsedLimit);
                    break;
                case "--lazer-path" when i + 1 < args.Length:
                    lazerPath = args[++i].Trim();
                    break;
            }
        }

        lazerPath = string.IsNullOrWhiteSpace(lazerPath)
            ? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "osu")
            : Path.GetFullPath(lazerPath);

        string sourceRealm = Path.Combine(lazerPath, "client.realm");
        if (!File.Exists(sourceRealm))
        {
            Console.Error.WriteLine($"client.realm not found at: {sourceRealm}");
            return 2;
        }

        string tempRealm = Path.Combine(
            Path.GetTempPath(),
            $"danser_client_copy_{Environment.ProcessId}_{Guid.NewGuid():N}.realm"
        );

        try
        {
            CopyLockedFile(sourceRealm, tempRealm);

            var config = new RealmConfiguration(tempRealm)
            {
                IsReadOnly = true,
                IsDynamic = true
            };

            using var realm = Realm.GetInstance(config);

            switch (command)
            {
                case "resolve-beatmap":
                    if (string.IsNullOrEmpty(md5) && string.IsNullOrEmpty(hash))
                    {
                        Console.Error.WriteLine("resolve-beatmap requires --md5 <md5> or --hash <sha256>");
                        return 1;
                    }

                    var resolved = ResolveBeatmap(realm, lazerPath, md5, hash);
                    if (resolved == null)
                    {
                        Console.Error.WriteLine("Beatmap not found in client.realm");
                        return 3;
                    }

                    Console.WriteLine(JsonSerializer.Serialize(resolved, JsonOptions));
                    return 0;

                case "list-replays":
                    var replays = ListReplays(realm, lazerPath, limit);
                    Console.WriteLine(JsonSerializer.Serialize(replays, JsonOptions));
                    return 0;

                case "list-skins":
                    var skins = ListSkins(realm);
                    Console.WriteLine(JsonSerializer.Serialize(skins, JsonOptions));
                    return 0;

                case "resolve-skin":
                    if (string.IsNullOrEmpty(name))
                    {
                        Console.Error.WriteLine("resolve-skin requires --name <skin_name>");
                        return 1;
                    }

                    var resolvedSkin = ResolveSkin(realm, lazerPath, name);
                    if (resolvedSkin == null)
                    {
                        Console.Error.WriteLine($"Skin not found in client.realm: {name}");
                        return 3;
                    }

                    Console.WriteLine(JsonSerializer.Serialize(resolvedSkin, JsonOptions));
                    return 0;

                default:
                    Console.Error.WriteLine($"Unknown command: {command}");
                    return 1;
            }
        }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"Error: {ex.Message}");
            return 1;
        }
        finally
        {
            CleanupTempRealm(tempRealm);
        }
    }

    private static void CopyLockedFile(string sourcePath, string destPath)
    {
        using var src = new FileStream(sourcePath, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete);
        using var dst = new FileStream(destPath, FileMode.Create, FileAccess.Write, FileShare.None);
        src.CopyTo(dst);
    }

    private static void CleanupTempRealm(string tempRealm)
    {
        TryDeleteFile(tempRealm);
        TryDeleteFile(tempRealm + ".lock");
        TryDeleteFile(tempRealm + ".note");
        TryDeleteDirectory(tempRealm + ".management");
    }

    private static void TryDeleteFile(string path)
    {
        try
        {
            if (File.Exists(path))
                File.Delete(path);
        }
        catch
        {
            // Best effort cleanup
        }
    }

    private static void TryDeleteDirectory(string path)
    {
        try
        {
            if (Directory.Exists(path))
                Directory.Delete(path, true);
        }
        catch
        {
            // Best effort cleanup
        }
    }

    private static string? FindClassName(Realm realm, params string[] candidates)
    {
        foreach (string candidate in candidates)
        {
            if (realm.Schema.TryFindObjectSchema(candidate, out _))
                return candidate;
        }

        return null;
    }

    private static string ResolveStoragePath(string lazerPath, string hash)
    {
        if (string.IsNullOrWhiteSpace(hash) || hash.Length < 2)
            return string.Empty;

        string normalized = hash.Trim().ToLowerInvariant();
        string twoLevel = Path.Combine(lazerPath, "files", normalized[..1], normalized[..2], normalized);
        if (File.Exists(twoLevel))
            return twoLevel;

        string oneLevel = Path.Combine(lazerPath, "files", normalized[..2], normalized);
        if (File.Exists(oneLevel))
            return oneLevel;

        return twoLevel;
    }

    private static ResolvedBeatmap? ResolveBeatmap(Realm realm, string lazerPath, string? md5, string? hash)
    {
        string? beatmapClass = FindClassName(realm, "Beatmap", "BeatmapInfo");
        if (beatmapClass == null)
            throw new InvalidOperationException("Beatmap schema not found in client.realm");

        IRealmObjectBase? matched = null;
        IRealmObjectBase? fallbackMatched = null;

        foreach (IRealmObjectBase bm in realm.DynamicApi.All(beatmapClass))
        {
            string? bmMd5 = GetString(bm, "MD5Hash");
            string? bmHash = GetString(bm, "Hash");

            bool isMatch = (!string.IsNullOrEmpty(md5) && string.Equals(bmMd5, md5, StringComparison.OrdinalIgnoreCase))
                        || (!string.IsNullOrEmpty(hash) && string.Equals(bmHash, hash, StringComparison.OrdinalIgnoreCase));

            if (!isMatch)
                continue;

            var set = GetObject(bm, "BeatmapSet");
            bool deletePending = set != null && GetBool(set, "DeletePending");
            if (!deletePending)
            {
                matched = bm;
                break;
            }

            fallbackMatched ??= bm;
        }

        matched ??= fallbackMatched;
        if (matched == null)
            return null;

        var beatmapSet = GetObject(matched, "BeatmapSet");
        var metadata = GetObject(matched, "Metadata");
        var author = metadata != null ? GetObject(metadata, "Author") : null;

        var filesMap = new Dictionary<string, string>(StringComparer.Ordinal);
        var filesLookup = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);

        if (beatmapSet != null)
        {
            foreach (var namedFile in GetObjectList(beatmapSet, "Files"))
            {
                string? filename = GetString(namedFile, "Filename");
                var fileObj = GetObject(namedFile, "File");
                string? fileHash = fileObj != null ? GetString(fileObj, "Hash") : null;

                if (string.IsNullOrWhiteSpace(filename) || string.IsNullOrWhiteSpace(fileHash))
                    continue;

                string normalizedName = filename.Replace('\\', '/');
                string physicalPath = ResolveStoragePath(lazerPath, fileHash);

                filesMap[normalizedName] = physicalPath;
                filesLookup[normalizedName] = physicalPath;
            }
        }

        string beatmapPath = GetString(matched, "Path") ?? string.Empty;
        string beatmapHash = GetString(matched, "Hash") ?? string.Empty;
        string beatmapMd5 = GetString(matched, "MD5Hash") ?? string.Empty;

        string osuPath = LookupFile(filesLookup, beatmapPath);
        if (string.IsNullOrEmpty(osuPath) && !string.IsNullOrEmpty(beatmapHash))
        {
            string candidate = ResolveStoragePath(lazerPath, beatmapHash);
            if (File.Exists(candidate))
                osuPath = candidate;
        }

        string audioFilename = metadata != null ? (GetString(metadata, "AudioFile") ?? string.Empty) : string.Empty;
        string backgroundFilename = metadata != null ? (GetString(metadata, "BackgroundFile") ?? string.Empty) : string.Empty;

        return new ResolvedBeatmap
        {
            Md5 = beatmapMd5,
            Hash = beatmapHash,
            OsuPath = osuPath,
            AudioPath = LookupFile(filesLookup, audioFilename),
            BackgroundPath = LookupFile(filesLookup, backgroundFilename),
            AudioFilename = audioFilename,
            BackgroundFilename = backgroundFilename,
            Artist = metadata != null ? (GetString(metadata, "Artist") ?? string.Empty) : string.Empty,
            Title = metadata != null ? (GetString(metadata, "Title") ?? string.Empty) : string.Empty,
            Difficulty = GetString(matched, "DifficultyName") ?? string.Empty,
            Creator = author != null ? (GetString(author, "Username") ?? string.Empty) : string.Empty,
            Files = filesMap
        };
    }

    private static string LookupFile(Dictionary<string, string> lookup, string? relativeName)
    {
        if (string.IsNullOrWhiteSpace(relativeName))
            return string.Empty;

        string normalized = relativeName.Replace('\\', '/').TrimStart('/');
        return lookup.TryGetValue(normalized, out string? path) ? path : string.Empty;
    }

    private static List<LazerReplay> ListReplays(Realm realm, string lazerPath, int limit)
    {
        string? scoreClass = FindClassName(realm, "Score", "ScoreInfo");
        if (scoreClass == null)
            throw new InvalidOperationException("Score schema not found in client.realm");

        string? beatmapClass = FindClassName(realm, "Beatmap", "BeatmapInfo");
        var beatmapByHash = new Dictionary<string, IRealmObjectBase>(StringComparer.OrdinalIgnoreCase);

        if (beatmapClass != null)
        {
            foreach (IRealmObjectBase bm in realm.DynamicApi.All(beatmapClass))
            {
                string? h = GetString(bm, "Hash");
                if (!string.IsNullOrWhiteSpace(h) && !beatmapByHash.ContainsKey(h))
                    beatmapByHash[h] = bm;
            }
        }

        var entries = new List<(DateTimeOffset Date, LazerReplay Replay)>();

        foreach (IRealmObjectBase score in realm.DynamicApi.All(scoreClass))
        {
            if (GetBool(score, "DeletePending"))
                continue;

            var ruleset = GetObject(score, "Ruleset");
            if (ruleset != null)
            {
                string? shortName = GetString(ruleset, "ShortName");
                long? onlineId = GetNullableLong(ruleset, "OnlineID");
                if (!string.Equals(shortName, "osu", StringComparison.OrdinalIgnoreCase) && onlineId != 0)
                    continue;
            }

            string? replayPath = null;
            foreach (var namedFile in GetObjectList(score, "Files"))
            {
                string? filename = GetString(namedFile, "Filename");
                var fileObj = GetObject(namedFile, "File");
                string? fileHash = fileObj != null ? GetString(fileObj, "Hash") : null;
                if (string.IsNullOrWhiteSpace(fileHash))
                    continue;

                string physicalPath = ResolveStoragePath(lazerPath, fileHash);
                if (!File.Exists(physicalPath))
                    continue;

                if (!string.IsNullOrEmpty(filename) && filename.EndsWith(".osr", StringComparison.OrdinalIgnoreCase))
                {
                    replayPath = physicalPath;
                    break;
                }

                replayPath ??= physicalPath;
            }

            if (string.IsNullOrEmpty(replayPath))
                continue;

            var bm = GetObject(score, "BeatmapInfo");
            string beatmapHash = GetString(score, "BeatmapHash") ?? string.Empty;
            if (bm == null && !string.IsNullOrEmpty(beatmapHash))
                beatmapByHash.TryGetValue(beatmapHash, out bm);

            string beatmapMd5 = bm != null ? (GetString(bm, "MD5Hash") ?? string.Empty) : string.Empty;
            var metadata = bm != null ? GetObject(bm, "Metadata") : null;
            var user = GetObject(score, "User");
            var date = GetDateTimeOffset(score, "Date");

            entries.Add((date, new LazerReplay
            {
                Hash = GetString(score, "Hash") ?? string.Empty,
                BeatmapMd5 = beatmapMd5,
                BeatmapHash = beatmapHash,
                ReplayPath = replayPath,
                Username = user != null ? (GetString(user, "Username") ?? string.Empty) : string.Empty,
                Artist = metadata != null ? (GetString(metadata, "Artist") ?? string.Empty) : string.Empty,
                Title = metadata != null ? (GetString(metadata, "Title") ?? string.Empty) : string.Empty,
                Difficulty = bm != null ? (GetString(bm, "DifficultyName") ?? string.Empty) : string.Empty,
                TotalScore = GetNullableLong(score, "TotalScore") ?? 0,
                MaxCombo = (int)(GetNullableLong(score, "MaxCombo") ?? 0),
                Accuracy = GetDouble(score, "Accuracy"),
                Date = date.ToString("O"),
                Mods = GetString(score, "Mods") ?? string.Empty
            }));
        }

        return entries
            .OrderByDescending(x => x.Date)
            .Take(limit)
            .Select(x => x.Replay)
            .ToList();
    }

    private static bool HasProp(IRealmObjectBase obj, string propName) =>
        obj.ObjectSchema != null && obj.ObjectSchema.TryFindProperty(propName, out _);

    private static string? GetString(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return null;

        try
        {
            return obj.DynamicApi.Get<string?>(propName);
        }
        catch
        {
            return null;
        }
    }

    private static bool GetBool(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return false;

        try
        {
            return obj.DynamicApi.Get<bool>(propName);
        }
        catch
        {
            return false;
        }
    }

    private static long? GetNullableLong(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return null;

        try
        {
            var val = obj.DynamicApi.Get<RealmValue>(propName);
            if (val.Type == RealmValueType.Int)
                return val.AsInt64();
            return null;
        }
        catch
        {
            return null;
        }
    }

    private static double GetDouble(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return 0;

        try
        {
            var val = obj.DynamicApi.Get<RealmValue>(propName);
            return val.Type switch
            {
                RealmValueType.Double => val.AsDouble(),
                RealmValueType.Float => val.AsFloat(),
                _ => 0
            };
        }
        catch
        {
            return 0;
        }
    }

    private static DateTimeOffset GetDateTimeOffset(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return DateTimeOffset.MinValue;

        try
        {
            return obj.DynamicApi.Get<DateTimeOffset>(propName);
        }
        catch
        {
            return DateTimeOffset.MinValue;
        }
    }

    private static IRealmObjectBase? GetObject(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return null;

        try
        {
            var val = obj.DynamicApi.Get<RealmValue>(propName);
            return val.Type == RealmValueType.Object ? val.AsIRealmObject() : null;
        }
        catch
        {
            return null;
        }
    }

    private static IEnumerable<IRealmObjectBase> GetObjectList(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            yield break;

        IList<IRealmObjectBase> list;
        try
        {
            list = obj.DynamicApi.GetList<IRealmObjectBase>(propName);
        }
        catch
        {
            yield break;
        }

        foreach (var item in list)
            yield return item;
    }

    private static string GetIdString(IRealmObjectBase obj, string propName)
    {
        if (!HasProp(obj, propName))
            return string.Empty;

        try
        {
            var val = obj.DynamicApi.Get<RealmValue>(propName);
            return val.Type switch
            {
                RealmValueType.Guid => val.AsGuid().ToString("D"),
                RealmValueType.String => val.AsString(),
                _ => string.Empty
            };
        }
        catch
        {
            return string.Empty;
        }
    }

    private sealed class SkinCandidate
    {
        public required IRealmObjectBase Obj { get; init; }
        public required string Id { get; init; }
        public required string RawName { get; init; }
        public required string Creator { get; init; }
        public required string DisplayName { get; set; }
        public required int FileCount { get; init; }
    }

    private static List<SkinCandidate> CollectSkins(Realm realm)
    {
        string? skinClass = FindClassName(realm, "Skin", "SkinInfo");
        if (skinClass == null)
            throw new InvalidOperationException("Skin schema not found in client.realm");

        var list = new List<SkinCandidate>();

        foreach (IRealmObjectBase skin in realm.DynamicApi.All(skinClass))
        {
            if (GetBool(skin, "DeletePending"))
                continue;

            int fileCount = 0;
            foreach (var namedFile in GetObjectList(skin, "Files"))
            {
                string? filename = GetString(namedFile, "Filename");
                var fileObj = GetObject(namedFile, "File");
                string? fileHash = fileObj != null ? GetString(fileObj, "Hash") : null;
                if (!string.IsNullOrWhiteSpace(filename) && !string.IsNullOrWhiteSpace(fileHash))
                    fileCount++;
            }

            if (fileCount == 0)
                continue;

            string rawName = (GetString(skin, "Name") ?? string.Empty).Trim();
            if (string.IsNullOrEmpty(rawName))
                continue;

            string id = GetIdString(skin, "ID");
            string creator = (GetString(skin, "Creator") ?? string.Empty).Trim();

            list.Add(new SkinCandidate
            {
                Obj = skin,
                Id = id,
                RawName = rawName,
                Creator = creator,
                DisplayName = rawName,
                FileCount = fileCount
            });
        }

        var nameCounts = list
            .GroupBy(s => s.RawName, StringComparer.OrdinalIgnoreCase)
            .ToDictionary(g => g.Key, g => g.Count(), StringComparer.OrdinalIgnoreCase);

        var usedDisplayNames = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var item in list)
        {
            if (nameCounts[item.RawName] > 1)
            {
                string candidate = !string.IsNullOrEmpty(item.Creator)
                    ? $"{item.RawName} ({item.Creator})"
                    : item.RawName;

                if (usedDisplayNames.Contains(candidate) || string.Equals(candidate, item.RawName, StringComparison.OrdinalIgnoreCase))
                {
                    string shortId = item.Id.Length >= 6 ? item.Id[..6] : item.Id;
                    candidate = $"{item.RawName} [{shortId}]";
                }

                item.DisplayName = candidate;
            }

            usedDisplayNames.Add(item.DisplayName);
        }

        return list;
    }

    private static List<LazerSkinInfo> ListSkins(Realm realm)
    {
        return CollectSkins(realm)
            .OrderBy(s => s.DisplayName, StringComparer.OrdinalIgnoreCase)
            .Select(s => new LazerSkinInfo
            {
                Id = s.Id,
                Name = s.DisplayName,
                Creator = s.Creator,
                FileCount = s.FileCount
            })
            .ToList();
    }

    private static ResolvedSkin? ResolveSkin(Realm realm, string lazerPath, string targetName)
    {
        var candidates = CollectSkins(realm);
        var match = candidates.FirstOrDefault(s => string.Equals(s.DisplayName, targetName, StringComparison.OrdinalIgnoreCase))
                 ?? candidates.FirstOrDefault(s => string.Equals(s.RawName, targetName, StringComparison.OrdinalIgnoreCase));

        if (match == null)
            return null;

        var filesMap = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var namedFile in GetObjectList(match.Obj, "Files"))
        {
            string? filename = GetString(namedFile, "Filename");
            var fileObj = GetObject(namedFile, "File");
            string? fileHash = fileObj != null ? GetString(fileObj, "Hash") : null;
            if (string.IsNullOrWhiteSpace(filename) || string.IsNullOrWhiteSpace(fileHash))
                continue;

            string normalizedName = filename.Replace('\\', '/').TrimStart('/');
            string physicalPath = ResolveStoragePath(lazerPath, fileHash);
            filesMap[normalizedName] = physicalPath;
        }

        return new ResolvedSkin
        {
            Id = match.Id,
            Name = match.DisplayName,
            Creator = match.Creator,
            Files = filesMap
        };
    }

    private sealed class LazerSkinInfo
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Creator { get; set; } = string.Empty;
        public int FileCount { get; set; }
    }

    private sealed class ResolvedSkin
    {
        public string Id { get; set; } = string.Empty;
        public string Name { get; set; } = string.Empty;
        public string Creator { get; set; } = string.Empty;
        public Dictionary<string, string> Files { get; set; } = new();
    }

    private sealed class ResolvedBeatmap
    {
        public string Md5 { get; set; } = string.Empty;
        public string Hash { get; set; } = string.Empty;
        public string OsuPath { get; set; } = string.Empty;
        public string AudioPath { get; set; } = string.Empty;
        public string BackgroundPath { get; set; } = string.Empty;
        public string AudioFilename { get; set; } = string.Empty;
        public string BackgroundFilename { get; set; } = string.Empty;
        public string Artist { get; set; } = string.Empty;
        public string Title { get; set; } = string.Empty;
        public string Difficulty { get; set; } = string.Empty;
        public string Creator { get; set; } = string.Empty;
        public Dictionary<string, string> Files { get; set; } = new();
    }

    private sealed class LazerReplay
    {
        public string Hash { get; set; } = string.Empty;
        public string BeatmapMd5 { get; set; } = string.Empty;
        public string BeatmapHash { get; set; } = string.Empty;
        public string ReplayPath { get; set; } = string.Empty;
        public string Username { get; set; } = string.Empty;
        public string Artist { get; set; } = string.Empty;
        public string Title { get; set; } = string.Empty;
        public string Difficulty { get; set; } = string.Empty;
        public long TotalScore { get; set; }
        public int MaxCombo { get; set; }
        public double Accuracy { get; set; }
        public string Date { get; set; } = string.Empty;
        public string Mods { get; set; } = string.Empty;
    }
}
