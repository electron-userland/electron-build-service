"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (_) try {
            if (f = 1, y && (t = y[op[0] & 2 ? "return" : op[0] ? "throw" : "next"]) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [0, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
var path = require("path");
var fs = require("fs-extra-p");
var os = require("os");
var Queue = require("bull");
var _a = require("builder-util/out/bundledTool"), computeEnv = _a.computeEnv, getLinuxToolsPath = _a.getLinuxToolsPath;
var HTTP2_HEADER_PATH = require("http2").constants.HTTP2_HEADER_PATH;
var loggerOptions = true;
if (process.env.PRETTY_PRINT_LOGS) {
    var pino = require("pino");
    var stream = process.stdout;
    var pretty = pino.pretty();
    pretty.pipe(stream);
    stream = pretty;
    loggerOptions = pino({ level: "info" }, stream);
}
var fastify = require("fastify")({
    http2: true,
    logger: loggerOptions,
});
var redisEndPoint = process.env.REDIS_ENDPOINT;
var buildQueue = new Queue("build-" + os.hostname(), "redis://" + redisEndPoint);
fastify.addContentTypeParser("application/octet-stream", function (request, done) {
    done();
});
var SUCCESS_RESPONSE = {};
var pendingAppArchiveDir = path.join(os.tmpdir(), "electron-build-server");
fastify.route({
    method: "POST",
    url: "/v1/build",
    handler: function (request, reply) {
        // save to temp file
        var archiveFile = path.join(pendingAppArchiveDir, process.pid.toString(16) + "-" + Date.now().toString(16) + "-" + Math.floor(Math.random() * 1024 * 1024).toString(16) + ".zst");
        var fileStream = fs.createWriteStream(archiveFile);
        var errorHandler = function (error) {
            console.error(error);
            reply
                .code(501)
                .send(error);
        };
        fileStream.on("error", errorHandler);
        var inputStream = request.req;
        inputStream.on("error", errorHandler);
        inputStream.on("end", function () {
            buildQueue.add({ app: archiveFile })
                .then(function (job) { return job.finished(); })
                .then(function (data) {
                if (data.error != null) {
                    reply.send(data);
                    return;
                }
                var artifacts = data.artifacts;
                var _loop_1 = function (file) {
                    request.req.stream.pushStream((_a = {}, _a[HTTP2_HEADER_PATH] = path.relative(data.outDirectory, file), _a), function (pushStream) {
                        pushStream.respondWithFile(file, {
                            onError: errorHandler,
                        });
                        // pushStream.respond({ ':status': 200 });
                        // pushStream.end('some pushed data');
                        if (file === artifacts[artifacts.length - 1]) {
                        }
                    });
                    reply.send({});
                    var _a;
                };
                for (var _i = 0, artifacts_1 = artifacts; _i < artifacts_1.length; _i++) {
                    var file = artifacts_1[_i];
                    _loop_1(file);
                }
            })
                .catch(errorHandler);
        });
        inputStream.pipe(fileStream);
    },
});
// on macOS GNU tar is required
function prepareBuildTools() {
    return __awaiter(this, void 0, void 0, function () {
        var linuxToolsPath;
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    if (!(process.platform === "darwin")) return [3 /*break*/, 2];
                    return [4 /*yield*/, getLinuxToolsPath()];
                case 1:
                    linuxToolsPath = _a.sent();
                    process.env.PATH = computeEnv(process.env.PATH, [path.join(linuxToolsPath, "bin")]);
                    process.env.DYLD_LIBRARY_PATH = computeEnv(process.env.DYLD_LIBRARY_PATH, [path.join(linuxToolsPath, "lib")]);
                    process.env.TAR_PATH = path.join(linuxToolsPath, "bin", "gtar");
                    return [3 /*break*/, 3];
                case 2:
                    process.env.TAR_PATH = "tar";
                    _a.label = 3;
                case 3: return [2 /*return*/];
            }
        });
    });
}
function main() {
    return __awaiter(this, void 0, void 0, function () {
        var builderPath, isSandboxed, concurrency;
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    buildQueue.on("error", function (error) {
                        console.error(error);
                    });
                    builderPath = path.join(__dirname, "builder.js");
                    isSandboxed = process.env.SANDBOXED_BUILD_PROCESS !== "false";
                    concurrency = isSandboxed ? (os.cpus().length + 1) : 1;
                    // clean queue on restart since in any case client task is cancelled on abort
                    return [4 /*yield*/, Promise.all([
                            buildQueue.empty(),
                            prepareBuildTools(),
                            fs.emptyDir(pendingAppArchiveDir),
                        ])];
                case 1:
                    // clean queue on restart since in any case client task is cancelled on abort
                    _a.sent();
                    buildQueue.process(concurrency, isSandboxed ? builderPath : require(builderPath));
                    fastify.listen(3000, function (error) {
                        if (error != null) {
                            throw error;
                        }
                        console.log("server listening on " + fastify.server.address().port + ", temp dir: " + pendingAppArchiveDir + ", concurrency: " + concurrency + ", redis endpoint: " + redisEndPoint + ", isSandboxed: " + isSandboxed + ", queue name: " + buildQueue.name);
                    });
                    return [2 /*return*/];
            }
        });
    });
}
main()
    .catch(function (error) {
    console.error(error.stack || error);
    process.exit(1);
});
