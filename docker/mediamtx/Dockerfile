# MediaMTX RTSP/RTMP/WebRTC Server for HelixQA
FROM docker.io/bluenviron/mediamtx:latest

# Copy configuration
COPY mediamtx.yml /mediamtx.yml

# Expose ports
# RTSP
EXPOSE 8554
# RTMP
EXPOSE 1935
# HLS
EXPOSE 8888
# WebRTC
EXPOSE 8889
# WebRTC ICE
EXPOSE 8189/udp
EXPOSE 8189/tcp
# Metrics
EXPOSE 9998

# Run MediaMTX
ENTRYPOINT ["/mediamtx"]
CMD ["/mediamtx.yml"]
