#!/usr/local/bin/ python3.9
import json
import os
import sys
import time
import warnings

from minio import Minio


def get_client():
    file = os.path.join(os.getenv("HOME"), "Desktop", "blog", "env.json")
    print("密钥文件", file)
    with open(file, 'r') as load_f:
        load_dict = json.load(load_f)
    print(load_dict)

    access_key = load_dict.get("accessKey")
    secret_key = load_dict.get("secretKey")
    print("使用密钥", access_key, secret_key)

    warnings.filterwarnings('ignore')

    client = Minio("192.168.50.209:9000", access_key=access_key, secret_key=secret_key, secure=False)
    return client


def get_type(ext):
    if ext in [".png", ".jpg", ".gif"]:
        return "image/" + ext.replace(".", "")
    return "image/jpg"


def get_name(ext):
    date = time.strftime("%Y%m%d%H%M%S", time.localtime())
    md_name = sys.argv[1]
    new_name = f"{md_name}_{date}{ext}"
    return new_name


def main():
    print("参数是", sys.argv)
    result = "Upload Success:\n"
    images = sys.argv[2:]
    bucket = "blog"
    minio_client = get_client()
    for image in images:
        file_type = os.path.splitext(image)[-1]
        new_file_name = get_name(file_type)
        content_type = get_type(file_type)
        print("新文件名", new_file_name, content_type)
        try:
            minio_client.fput_object(bucket_name=bucket, object_name=new_file_name, file_path=image,
                                     content_type=content_type)
            result = result + f"http://inksnw.asuscomm.com:3001/{bucket}/{new_file_name}\n"

        except Exception as e:
            print("出错了", e)
            return
    print(result)


if __name__ == '__main__':
    main()
