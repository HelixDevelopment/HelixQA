# `helixqa-lpips`

Triton Inference Server model config for LPIPS. Consumed by `pkg/vision/perceptual/lpips` on the Go side.

**Status:** operator-action. Go client code-complete + 95% tested (M52).

## Triton model config

Place LPIPS ONNX at `/models/lpips/1/model.onnx` and `/models/lpips/config.pbtxt`:

```
name: "lpips"
platform: "onnxruntime_onnx"
max_batch_size: 1

input [
  { name: "IMAGE_A", data_type: TYPE_STRING, dims: [ 1 ] },
  { name: "IMAGE_B", data_type: TYPE_STRING, dims: [ 1 ] }
]

output [
  { name: "DISTANCE", data_type: TYPE_FP32, dims: [ 1 ] }
]

instance_group [ { kind: KIND_GPU, count: 1 } ]
```

## Python backend

```python
import triton_python_backend_utils as pb_utils
import base64, io, numpy as np, torch, lpips
from PIL import Image
from torchvision import transforms

class TritonPythonModel:
    def initialize(self, args):
        self.model = lpips.LPIPS(net='alex').cuda()
        self.preprocess = transforms.Compose([
            transforms.Resize(256), transforms.CenterCrop(224),
            transforms.ToTensor(),
            transforms.Normalize([0.5]*3, [0.5]*3),
        ])

    def execute(self, requests):
        responses = []
        for req in requests:
            a = pb_utils.get_input_tensor_by_name(req, "IMAGE_A").as_numpy()
            b = pb_utils.get_input_tensor_by_name(req, "IMAGE_B").as_numpy()
            t_a = self._decode(a[0][0]); t_b = self._decode(b[0][0])
            with torch.no_grad():
                d = self.model(t_a, t_b).item()
            out = pb_utils.Tensor("DISTANCE", np.array([d], dtype=np.float32))
            responses.append(pb_utils.InferenceResponse(output_tensors=[out]))
        return responses

    def _decode(self, b64):
        img = Image.open(io.BytesIO(base64.b64decode(b64))).convert("RGB")
        return self.preprocess(img).unsqueeze(0).cuda()
```

## Acceptance

1. `POST /v2/models/lpips/infer` with identical images → `DISTANCE ≤ 0.05`.
2. HelixQA Go client's `distanceToSimilarity` maps `0.05` → `0.9` similarity (default `MaxDistance=1.0`).
3. Full `pkg/vision/perceptual/lpips_test.go` passes against the live endpoint if the test hooks are pointed at it.
