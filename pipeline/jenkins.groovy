pipeline {
    agent {
        kubernetes {
            label 'golang-agent'
            defaultContainer 'golang'
            yaml """
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: golang
    image: golang:1.23
    command:
    - cat
    tty: true
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  - name: docker
    image: docker:27.2.0-dind
    command:
    - dockerd-entrypoint.sh
    args:
    - "--host=tcp://0.0.0.0:2375"
    - "--host=unix:///var/run/docker.sock"
    securityContext:
      privileged: true
    env:
    - name: DOCKER_TLS_CERTDIR
      value: ""
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  - name: jnlp
    image: jenkins/inbound-agent:latest
    env:
    - name: JENKINS_AGENT_WORKDIR
      value: /home/jenkins/agent
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  volumes:
  - name: workspace-volume
    emptyDir: {}
"""
        }
    }

    parameters {
        choice(name: 'OS', choices: ['linux'], description: 'Target OS')
        choice(name: 'ARCH', choices: ['amd64', 'arm64'], description: 'Target architecture')
    }

    environment {
        DOCKER_CLI_EXPERIMENTAL = "enabled"
        DOCKER_BUILDKIT = "1"
        REGISTRY = "ghcr.io"
        REPO = "armrcode/kbot"
        IMAGE = "${env.REGISTRY}/${env.REPO}"
    }

    stages {
        stage('Checkout SCM') {
            steps {
                checkout scm
                sh 'git config --global --add safe.directory ${WORKSPACE}'
            }
        }

        stage('Lint') {
            steps {
                container('golang') {
                    sh '''
                        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.60.3
                        ./bin/golangci-lint run --timeout=5m -v
                    '''
                }
            }
        }

        stage('Test') {
            steps {
                container('golang') {
                    sh 'go test ./...'
                }
            }
        }

        stage('Build Binary') {
            steps {
                container('golang') {
                    sh "make build TARGETOS=${params.OS} TARGETARCH=${params.ARCH}"
                }
            }
        }

        stage('Docker Build & Push') {
            steps {
                container('docker') {
                    withCredentials([string(credentialsId: 'GHCR_PAT', variable: 'CR_PAT')]) {
                        withEnv(["TARGETARCH=${params.TARGETARCH}"]) {
                        sh '''
                            # Добавляем безопасную директорию внутри контейнера
                            git config --global --add safe.directory /home/jenkins/agent/workspace/Pipeline_demo

                            COMMIT_HASH=$(git rev-parse --short HEAD)
                            OS=linux
                            ARCH=${TARGETARCH:-amd64}
                            VERSION="v1.0.0-${COMMIT_HASH}-${OS}-${ARCH}"

                            echo "Building image $IMAGE:$VERSION for ${OS}/${ARCH}"

                            echo $CR_PAT | docker login ghcr.io -u ${GITHUB_ACTOR:-jenkins} --password-stdin
                            docker buildx create --use --name multiarch || true
                            docker buildx build \
                                --platform linux/${ARCH} \
                                -t $IMAGE:$VERSION \
                                -t $IMAGE:latest \
                                --push .
                            echo $VERSION > .image_version
                        '''
                        }
                    }
                }
            }
        }

        stage('Update Helm Chart') {
            steps {
                container('golang') {
                    withCredentials([string(credentialsId: 'GHCR_PAT', variable: 'CR_PAT')]) {
                        withEnv(["TARGETARCH=${params.TARGETARCH}"]) {
                            sh '''
                            # Устанавливаем yq, если его нет
                            if ! command -v yq &> /dev/null; then
                            echo "Installing yq..."
                            curl -L https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -o /usr/local/bin/yq
                            chmod +x /usr/local/bin/yq
                            fi

                            VERSION=$(cat .image_version)
                            OS=linux
                            ARCH=${TARGETARCH:-amd64}

                            FULL_TAG="${VERSION}-${OS}-${ARCH}"

                            echo "Updating helm/values.yaml with tag ${FULL_TAG} and arch ${ARCH}"

                            # Обновляем Helm values.yaml
                            yq eval ".image.tag = strenv(FULL_TAG)" --inplace helm/values.yaml
                            yq eval ".image.arch = strenv(ARCH)" --inplace helm/values.yaml

                            git config user.name "jenkins"
                            git config user.email "jenkins@local"
                            git add helm/values.yaml
                            git commit -m "Update Helm image tag to ${FULL_TAG} for arch ${ARCH}" || echo "No changes to commit"
                            git push https://$CR_PAT@github.com/ARmrCode/kbot.git HEAD:main
                            '''
                        }
                    }   
                }
            }
        }
    }
}
