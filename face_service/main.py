from fastapi import FastAPI
from pydantic import BaseModel
from typing import List
import numpy as np
import base64
import cv2
from insightface.app import FaceAnalysis

app = FastAPI()

# ─────────────────────────────────────────
# Load model InsightFace buffalo_l
# Pertama kali: download otomatis ~300MB ke C:\Users\<nama>\.insightface\
# Selanjutnya: load dari cache, cepat
# ─────────────────────────────────────────
print("Loading InsightFace model buffalo_l...")
face_app = FaceAnalysis(
    name='buffalo_l',
    providers=['CPUExecutionProvider']
)
face_app.prepare(ctx_id=0, det_size=(640, 640))
print("Model loaded successfully.")

THRESHOLD = 0.60  # cosine similarity minimum untuk dianggap match


# ─────────────────────────────────────────
# Helper: base64 string → OpenCV image (numpy array)
# ─────────────────────────────────────────
def base64_to_image(b64_string: str):
    # Hapus prefix data:image/...;base64, jika ada (dari browser/FE)
    if "," in b64_string:
        b64_string = b64_string.split(",")[1]
    img_bytes = base64.b64decode(b64_string)
    np_arr = np.frombuffer(img_bytes, np.uint8)
    img = cv2.imdecode(np_arr, cv2.IMREAD_COLOR)
    return img


# ─────────────────────────────────────────
# GET /health — cek service jalan
# ─────────────────────────────────────────
@app.get("/health")
async def health():
    return {"status": "ok", "model": "buffalo_l", "threshold": THRESHOLD}


# ─────────────────────────────────────────
# POST /detect
# Terima foto base64, return embedding 512 dimensi
#
# Request: {"image": "<base64 string>"}
# Response sukses: {
#   "detected": true,
#   "face_count": 1,
#   "embedding": [0.123, -0.456, ...] // 512 angka
# }
# Response gagal: {
#   "detected": false,
#   "face_count": 0,
#   "error": "pesan error"
# }
# ─────────────────────────────────────────
class DetectRequest(BaseModel):
    image: str

@app.post("/detect")
async def detect(req: DetectRequest):
    try:
        img = base64_to_image(req.image)
        if img is None:
            return {"detected": False, "face_count": 0,
                    "error": "gagal decode gambar — pastikan base64 valid"}

        faces = face_app.get(img)
        face_count = len(faces)

        if face_count == 0:
            return {"detected": False, "face_count": 0,
                    "error": "tidak ada wajah terdeteksi"}

        if face_count > 1:
            return {"detected": False, "face_count": face_count,
                    "error": "terdeteksi lebih dari satu wajah"}

        # Tepat 1 wajah — ambil embedding vektor 512 dimensi
        embedding = faces[0].embedding.tolist()

        return {
            "detected": True,
            "face_count": 1,
            "embedding": embedding  # list 512 float
        }

    except Exception as e:
        return {"detected": False, "face_count": 0, "error": str(e)}


# ─────────────────────────────────────────
# POST /compare
# Bandingkan dua embedding dengan cosine similarity
#
# Request: {
#   "embedding_new": [512 float],     // dari foto live saat absen
#   "embedding_stored": [512 float]   // dari DB (hasil registrasi)
# }
# Response: {
#   "similarity": 0.7234,  // 0.0 - 1.0
#   "match": true          // true jika >= 0.60
# }
# ─────────────────────────────────────────
class CompareRequest(BaseModel):
    embedding_new: List[float]
    embedding_stored: List[float]

@app.post("/compare")
async def compare(req: CompareRequest):
    try:
        e1 = np.array(req.embedding_new, dtype=np.float32)
        e2 = np.array(req.embedding_stored, dtype=np.float32)

        norm1 = np.linalg.norm(e1)
        norm2 = np.linalg.norm(e2)

        if norm1 == 0 or norm2 == 0:
            return {"similarity": 0.0, "match": False,
                    "error": "embedding tidak valid (norm = 0)"}

        # Cosine similarity
        similarity = float(np.dot(e1, e2) / (norm1 * norm2))
        similarity = round(similarity, 4)

        return {
            "similarity": similarity,
            "match": similarity >= THRESHOLD
        }

    except Exception as e:
        return {"similarity": 0.0, "match": False, "error": str(e)}