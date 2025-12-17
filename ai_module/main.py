import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from transformers import CLIPProcessor, CLIPModel
from PIL import Image
import requests
from io import BytesIO
import torch
import logging
import httpx
from fastapi.responses import JSONResponse

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize Model (Load once at startup)
MODEL_NAME = "openai/clip-vit-base-patch32"
model = CLIPModel.from_pretrained(MODEL_NAME)
processor = CLIPProcessor.from_pretrained(MODEL_NAME)
IMAGE_BASE_URL = "http://localhost:8081"

app = FastAPI()

class ImageRequest(BaseModel):
    file_path: str

@app.post("/vectorize")
async def vectorize_image(req: ImageRequest):
    try:
        url = f"{IMAGE_BASE_URL}{req.file_path}"

        async with httpx.AsyncClient(timeout=httpx.Timeout(30.0)) as client:
            response = await client.get(url)
            response.raise_for_status()
        
        image = Image.open(BytesIO(response.content))
        if image.mode != 'RGB':
            image = image.convert('RGB')
        inputs = processor(images=image, return_tensors="pt")
        with torch.no_grad():
            features = model.get_image_features(**inputs)
        
        # Return the 512-dim vector
        return {"vector": features.squeeze().tolist()}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

# Run with: uvicorn vectorizer_service:app --port 5000
if __name__ == "__main__":
    
    uvicorn.run(app, host="0.0.0.0", port=5000)