import os
import sys
import time
import warnings

from minio import Minio

url = "192.168.50.209:9000"
accessKey = os.environ.get("accessKey")
secretKey = os.environ.get("secretKey")
bucket = "blog"

warnings.filterwarnings('ignore')
images = sys.argv[2:]
minioClient = Minio(url, access_key=accessKey, secret_key=secretKey, secure=False)
result = "Upload Success:\n"

print("参数是", sys.argv)


def get_type(ext):
    if ext in [".png", ".jpg", ".gif"]:
        return "image/" + file_type.replace(".", "")
    return "image/jpg"


def get_name(ext):
    date = time.strftime("%Y%m%d%H%M%S", time.localtime())
    md_name = sys.argv[1]
    new_name = f"{md_name}_{date}{ext}"
    return new_name


for image in images:
    file_type = os.path.splitext(image)[-1]
    new_file_name = get_name(file_type)
    content_type = get_type(file_type)
    print("新文件名", new_file_name, content_type)
    minioClient.fput_object(bucket_name=bucket, object_name=new_file_name, file_path=image,
                            content_type=content_type)
    result = result + f"http://{url}/{bucket}/{new_file_name}\n"

print(result)
